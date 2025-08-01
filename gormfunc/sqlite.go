package gormfunc

import (
	"errors"
	"fmt"
	"gotulua/boolfunc"
	"gotulua/errorhandlefunc"
	"gotulua/i18nfunc"
	"gotulua/statefunc"
	"gotulua/syncfunc"
	"gotulua/timefunc"
	"gotulua/typesfunc"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/Shopify/go-lua"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	PrimaryKeyField = "id"

	// System metadata table name
	SysMetaTable = "table_metadata"

	// Special field types
	// TypeDate     = "DATE"
	// TypeTime     = "TIME"
	// TypeDateTime = "DATETIME"
	// TypeBoolean  = "BOOLEAN"
	// TypeInteger  = "INTEGER"
	// TypeReal     = "REAL"
	// TypeText     = "TEXT"
)

// type FilteredField struct {
// 	Field  string
// 	Filter string
// }

// Table represents a table with optional filters and selected columns
type Table struct {
	db                 *gorm.DB
	Name               string
	filterByField      string // Optional field to filter by
	plainFilter        string // Optional plain filter string
	rangeFilter        []interface{}
	Columns            []string
	orderBy            string
	defaultFieldValues map[string]interface{}
	filteredFields     map[string]string
	fieldTypes         map[string]string // Maps field names to their types
	Rows               *Rowset
	XRecord            Record
	OnAfterInsert      string
	OnAfterUpdate      string
	OnAfterDelete      string
}

// TableWrapper wraps a gormfunc.Table for Lua
type TableWrapper struct {
	Table *Table
}

// TableMetadata represents a field's metadata in the system table
type TableMetadata struct {
	ID           int64  `gorm:"primaryKey"`
	TableName    string `gorm:"not null;index:idx_table_field"`
	FieldName    string `gorm:"not null;index:idx_table_field"`
	ActualType   string `gorm:"not null"` // The actual SQLite type (TEXT, INTEGER, etc.)
	LogicalType  string `gorm:"not null"` // Our logical type (DATE, TIME, DATETIME, BOOLEAN, etc.)
	IsNullable   bool   `gorm:"not null;default:true"`
	DefaultValue string
	Temporary    bool `gorm:"not null;default:false"`
}

type TableMetadataWrapper struct {
	meta TableMetadata
	Drop bool
}

// CreateDB creates a new SQLite database with system metadata table
func CreateDB(dbPath string) (*gorm.DB, error) {
	// Check if file already exists
	if _, err := os.Stat(dbPath); err == nil {
		return OpenDB(dbPath), nil
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	// Create database connection which will create the file
	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, errors.New(i18nfunc.T("error.db_create_failed", nil))
	}

	// Create the system metadata table
	err = db.AutoMigrate(&TableMetadata{})
	if err != nil {
		return nil, errors.New(i18nfunc.T("error.db_metadata_create_failed", nil))
	}
	clearTempMeta(db)
	return db, nil
}

// OpenDB initializes and returns a GORM DB connection
func OpenDB(dbName string) *gorm.DB {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(sqlite.Open(dbName), gormConfig)
	if err != nil {
		log.Fatal(i18nfunc.T("error.db_open_failed", map[string]interface{}{
			"Name": dbName,
		}))
	}
	clearTempMeta(db)
	return db
}

// CloseDB closes the database connection
func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func clearTempMeta(db *gorm.DB) error {
	db.Where("temporary = ?", true).Delete(&TableMetadata{})
	return nil
}

// CreateTable creates a new table in the database with the specified name and description.
// The description is a string describing the fields, using the format "n::FieldName;t::FieldType;l::Length|..."
// For example: "n::Name;t::Text;l::100|n::Age;t::Integer"
// If a table with the given name already exists and openIfExists is true, it opens and returns the table.
// Otherwise, it throws an error if the table exists and openIfExists is false.
// The function also stores metadata for special field types (Boolean, Date, Time, DateTime).
func CreateTable(db *gorm.DB, name, structure string, openIfExists bool, temporary bool) *Table {
	//"n::Name;t::Text;l::100"
	if name == SysMetaTable {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_can_not_create_sysmeta", map[string]interface{}{
			"Name": name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	if tableExists(db, name) {
		if openIfExists {
			return OpenTable(db, name)
		} else {
			errorhandlefunc.ThrowError(i18nfunc.T("error.table_already_exists", map[string]interface{}{
				"Name": name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
	}
	var createTable string
	if temporary {
		createTable = "CREATE TEMP TABLE IF NOT EXISTS " + name + " ( id INTEGER PRIMARY KEY AUTOINCREMENT"
	} else {
		createTable = "CREATE TABLE IF NOT EXISTS " + name + " ( id INTEGER PRIMARY KEY AUTOINCREMENT"
	}
	fields := strings.Split(structure, "|")

	// Store metadata for special types
	var metadata []TableMetadata

	for _, field := range fields {
		parts := strings.Split(field, ";")
		var fieldName, fieldType, fieldLength string
		for _, part := range parts {
			params := strings.Split(part, "::")
			if len(params) == 2 {
				switch params[0] {
				case "n":
					fieldName = params[1]
				case "t":
					fieldType = params[1]
				case "l":
					fieldLength = params[1]
				}
			}
		}
		if fieldName != "" {
			var actualType, logicalType, defaultValue string
			switch fieldType {
			case "Text":
				actualType = "TEXT"
				if fieldLength != "" {
					createTable += ", " + fieldName + " TEXT(" + fieldLength + ")"
				} else {
					createTable += ", " + fieldName + " TEXT"
				}
				createTable += " DEFAULT ''"
			case "Integer":
				actualType = "INTEGER"
				createTable += ", " + fieldName + " INTEGER" + " DEFAULT 0"
				defaultValue = "0"
			case "Float":
				actualType = "REAL"
				createTable += ", " + fieldName + " REAL" + " DEFAULT 0.0"
				defaultValue = "0.0"
			case "Boolean":
				actualType = "INTEGER"
				logicalType = typesfunc.TypeBoolean
				createTable += ", " + fieldName + " INTEGER" + " DEFAULT 0"
				defaultValue = "0"
			case "Date":
				actualType = "TEXT"
				logicalType = typesfunc.TypeDate
				createTable += ", " + fieldName + " TEXT(10)" + " DEFAULT ''"
				defaultValue = ""
			case "Time":
				actualType = "TEXT"
				logicalType = typesfunc.TypeTime
				createTable += ", " + fieldName + " TEXT(8)" + " DEFAULT ''"
				defaultValue = ""
			case "DateTime":
				actualType = "TEXT"
				logicalType = typesfunc.TypeDateTime
				createTable += ", " + fieldName + " TEXT(19)" + " DEFAULT ''"
				defaultValue = ""
			default:
				errorhandlefunc.ThrowError(i18nfunc.T("error.db_invalid_field_type", map[string]interface{}{
					"Field": fieldName,
					"Type":  fieldType,
				}), errorhandlefunc.ErrorTypeScript, true)
				return nil
			}

			// Add field info to metadata
			metadata = append(metadata, TableMetadata{
				TableName:    name,
				FieldName:    fieldName,
				ActualType:   actualType,
				LogicalType:  logicalType,
				IsNullable:   false,
				DefaultValue: defaultValue,
				Temporary:    temporary,
			})
		}
	}
	createTable += ")"

	// Create the table
	result := db.Exec(createTable)
	if result.Error != nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_table_create_failed", map[string]interface{}{
			"Name": name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}

	// Store metadata for special types
	if len(metadata) > 0 {
		result = db.Create(&metadata)
		if result.Error != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_metadata_store_failed", map[string]interface{}{
				"Name": name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
	}

	return OpenTable(db, name)
}

func AlterTable(db *gorm.DB, name, structure string) *Table {
	//structure = "drop::ProjectId|add::TaskId;t::Integer"
	if name == SysMetaTable {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_can_not_create_sysmeta", map[string]interface{}{
			"Name": name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	if !tableExists(db, name) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.table_not_exists", map[string]interface{}{
			"Name": name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	var alterTable []string
	fields := strings.Split(structure, "|")

	// Store metadata for special types
	var metadata []TableMetadataWrapper

	for _, field := range fields {
		parts := strings.Split(field, ";")
		var dropField, addField, fieldType, fieldLength string
		for _, part := range parts {
			params := strings.Split(part, "::")
			if len(params) == 2 {
				switch params[0] {
				case "drop":
					dropField = params[1]
				case "add":
					addField = params[1]
				case "t":
					fieldType = params[1]
				case "l":
					fieldLength = params[1]
				}
			}
		}
		if dropField != "" {
			alterTable = append(alterTable, "ALTER TABLE "+name)
			alterTable[len(alterTable)-1] += " DROP COLUMN " + dropField
			metadata = append(metadata, TableMetadataWrapper{
				meta: TableMetadata{
					TableName: name,
					FieldName: dropField,
				},
				Drop: true,
			})
		}
		if addField != "" {
			alterTable = append(alterTable, "ALTER TABLE "+name)
			alterTable[len(alterTable)-1] += " ADD COLUMN " //+ addField
			var actualType, logicalType, defaultValue string
			switch fieldType {
			case "Text":
				actualType = "TEXT"
				if fieldLength != "" {
					alterTable[len(alterTable)-1] += " " + addField + " TEXT(" + fieldLength + ")"
				} else {
					alterTable[len(alterTable)-1] += " " + addField + " TEXT"
				}
				alterTable[len(alterTable)-1] += " DEFAULT ''"
				defaultValue = ""
			case "Integer":
				actualType = "INTEGER"
				alterTable[len(alterTable)-1] += " " + addField + " INTEGER" + " DEFAULT 0"
				defaultValue = "0"
			case "Float":
				actualType = "REAL"
				alterTable[len(alterTable)-1] += " " + addField + " REAL" + " DEFAULT 0.0"
				defaultValue = "0.0"
			case "Boolean":
				actualType = "INTEGER"
				logicalType = typesfunc.TypeBoolean
				alterTable[len(alterTable)-1] += " " + addField + " INTEGER" + " DEFAULT 0"
				defaultValue = "0"
			case "Date":
				actualType = "TEXT"
				logicalType = typesfunc.TypeDate
				alterTable[len(alterTable)-1] += " " + addField + " TEXT(10)" + " DEFAULT ''"
				defaultValue = ""
			case "Time":
				actualType = "TEXT"
				logicalType = typesfunc.TypeTime
				alterTable[len(alterTable)-1] += " " + addField + " TEXT(8)" + " DEFAULT ''"
				defaultValue = ""
			case "DateTime":
				actualType = "TEXT"
				logicalType = typesfunc.TypeDateTime
				alterTable[len(alterTable)-1] += " " + addField + " TEXT(19)" + " DEFAULT ''"
				defaultValue = ""
			default:
				errorhandlefunc.ThrowError(i18nfunc.T("error.db_invalid_field_type", map[string]interface{}{
					"Field": addField,
					"Type":  fieldType,
				}), errorhandlefunc.ErrorTypeScript, true)
				return nil
			}

			// Add field info to metadata
			metadata = append(metadata, TableMetadataWrapper{
				meta: TableMetadata{
					TableName:    name,
					FieldName:    addField,
					ActualType:   actualType,
					LogicalType:  logicalType,
					IsNullable:   false,
					DefaultValue: defaultValue,
				},
				Drop: false,
			})
		}
	}
	//alterTable += ")"

	// Create the table
	var result *gorm.DB
	tx := db.Begin()
	for _, sql := range alterTable {
		result = tx.Exec(sql)
		if result.Error != nil {
			tx.Rollback()
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_table_create_failed", map[string]interface{}{
				"Name": name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
	}

	// Store metadata for special types
	if len(metadata) > 0 {
		result = alterMetadata(tx, metadata)
		if result.Error != nil {
			tx.Rollback()
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_metadata_store_failed", map[string]interface{}{
				"Name": name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
	}
	tx.Commit()

	return OpenTable(db, name)
}

func alterMetadata(db *gorm.DB, metadata []TableMetadataWrapper) *gorm.DB {
	if len(metadata) == 0 {
		return db
	}
	for _, meta := range metadata {
		if meta.Drop {
			result := db.Where("table_name = ? AND field_name = ?", meta.meta.TableName, meta.meta.FieldName).Delete(&TableMetadata{
				TableName: meta.meta.TableName,
				FieldName: meta.meta.FieldName,
			})
			if result.Error != nil {
				return result
			}
		} else {
			result := db.Create(&TableMetadata{
				TableName:    meta.meta.TableName,
				FieldName:    meta.meta.FieldName,
				ActualType:   meta.meta.ActualType,
				LogicalType:  meta.meta.LogicalType,
				IsNullable:   meta.meta.IsNullable,
				DefaultValue: meta.meta.DefaultValue,
			})
			if result.Error != nil {
				return result
			}
		}
	}
	return db
}

// OpenTable creates a new Table object
func OpenTable(db *gorm.DB, name string) *Table {
	statefunc.ClearErrors()
	t := Table{
		db:                 db,
		Name:               name,
		filterByField:      "",
		plainFilter:        "",
		rangeFilter:        []interface{}{},
		Columns:            []string{},
		defaultFieldValues: make(map[string]interface{}),
		fieldTypes:         make(map[string]string),
		filteredFields:     make(map[string]string),
	}
	rows, err := db.Raw("PRAGMA table_info(" + name + ")").Rows()
	if err != nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_table_info_failed", map[string]interface{}{
			"Name": name,
		}), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_table_scan_failed", map[string]interface{}{
				"Name": name,
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
		t.Columns = append(t.Columns, name)
	}
	if !t.fillFieldsMeta() {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_scan_failed", nil), errorhandlefunc.ErrorTypeScript, true)
		return nil
	}
	return &t
}

// SetFilter sets a filter for the table by field and plain filter string
func (t *Table) SetFilter(field, filter string) *Table {
	t.rangeFilter = []interface{}{}
	if t.filteredFields == nil {
		t.filteredFields = make(map[string]string)
	}
	t.filteredFields[field] = filter
	return t
}

// SetRangeFilter sets a range filter for the table
func (t *Table) SetRangeFilter(field string, min, max interface{}) *Table {
	t.rangeFilter = []interface{}{min, max}
	t.filterByField = field
	t.plainFilter = ""
	return t
}

// OrderBy sets the ORDER BY clause (chainable)
func (t *Table) OrderBy(order string) *Table {
	t.orderBy = order
	return t
}

// Insert inserts a new record into the table using a map of field names to values
func (t *Table) Insert(fields map[string]interface{}, id *int64) bool {
	var cols []string
	var placeholders []string
	var vals []interface{}
	statefunc.ClearErrors()
	*id = 0
	for k, v := range t.defaultFieldValues {
		if k != PrimaryKeyField {
			value, exists := fields[k]
			if exists {
				var ok bool
				v, ok = t.fieldUserFormatToInternalFormat(k, value, "")
				if !ok {
					return false
				}
			}
			cols = append(cols, "\""+k+"\"")
			placeholders = append(placeholders, "?")
			vals = append(vals, v)
		}
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t.Name, strings.Join(cols, ","), strings.Join(placeholders, ","))
	result := t.db.Exec(query, vals...)
	if result.Error != nil {
		statefunc.SetLastErrorText(result.Error.Error())
		return false
	}

	// Get the last inserted ID using a struct to ensure proper scanning
	type LastID struct {
		ID int64 `gorm:"column:id"`
	}
	var lastID LastID
	if err := t.db.Raw("SELECT last_insert_rowid() as id").Scan(&lastID).Error; err != nil {
		statefunc.SetLastErrorText(err.Error())
		return false
	}
	r := t.getRecordById(lastID.ID)
	if r == nil {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_row_not_found", map[string]interface{}{
			"ID": lastID.ID,
		}), errorhandlefunc.ErrorTypeScript, true)
		return false
	}
	if len(t.Rows.Rows) > 0 && t.Rows.Rows[len(t.Rows.Rows)-1][PrimaryKeyField] == 0 {
		t.Rows.Rows[len(t.Rows.Rows)-1] = r
	} else {
		t.Rows.Rows = append(t.Rows.Rows, r)
		t.Rows.Pos = len(t.Rows.Rows) - 1
	}
	*id = lastID.ID
	if t.OnAfterInsert != "" {
		t.runOnAfterInsert()
	}
	return true
}

// Update updates an existing record in the table by ID using a map of field names to values
func (t *Table) Update(id int64, fields Record) bool {
	var setClauses []string
	var vals []interface{}
	statefunc.ClearErrors()
	if id < 1 {
		return false
	}
	if t.OnAfterUpdate != "" {
		t.XRecord = t.getRecordById(id)
		if t.XRecord == nil {
			return false
		}
	}
	for k, v := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", "\""+k+"\""))
		v, ok := t.fieldUserFormatToInternalFormat(k, v, "")
		if !ok {
			return false
		}
		vals = append(vals, v)
	}
	vals = append(vals, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE ID = ?", t.Name, strings.Join(setClauses, ", "))
	result := t.db.Exec(query, vals...).Error == nil
	if !result {
		t.XRecord = nil
		statefunc.SetLastErrorText(t.db.Error.Error())
		return false
	}
	r := t.getRecordById(id)
	if r == nil {
		return false
	}
	t.Rows.Rows[t.Rows.Pos] = r
	if t.OnAfterUpdate != "" {
		t.runOnAfterUpdate()
	}
	return true
}

// delete deletes a record by ID from the table
func (t *Table) delete(id interface{}) bool {
	statefunc.ClearErrors()
	if t.OnAfterDelete != "" {
		t.XRecord = t.getRecordById(id)
	}
	result := t.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE ID = ?", t.Name), id).Error == nil
	if !result {
		t.XRecord = nil
		statefunc.SetLastErrorText(t.db.Error.Error())
		return false
	}
	if t.OnAfterDelete != "" {
		t.runOnAfterDelete()
	}
	return true
}

// SetOnAfterDelete sets the function to be called after a record is deleted
func (t *Table) SetOnAfterDelete(funcName string) {
	t.OnAfterDelete = funcName
}

// SetOnAfterUpdate sets the function to be called after a record is updated
func (t *Table) SetOnAfterUpdate(funcName string) {
	t.OnAfterUpdate = funcName
}

// SetOnAfterInsert sets the function to be called after a record is inserted
func (t *Table) SetOnAfterInsert(funcName string) {
	t.OnAfterInsert = funcName
}

func (t *Table) runOnAfterInsert() {
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	statefunc.L.Global(t.OnAfterInsert)
	if !statefunc.L.IsFunction(-1) {
		//fmt.Printf("Lua global '%s' is not a function (type: %v)\n", t.OnAfterInsert, statefunc.L.TypeOf(-1))
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": t.OnAfterInsert,
		}), errorhandlefunc.ErrorTypeScript, true)
		statefunc.L.Pop(1)
		return
	}
	// Push the first argument (wrapper)
	wrapper := &TableWrapper{Table: t}
	statefunc.L.PushUserData(wrapper)
	statefunc.L.PushString("TableMT")
	statefunc.L.RawGet(lua.RegistryIndex)
	if statefunc.L.IsNil(-1) {
		statefunc.L.Pop(1)
		statefunc.L.Pop(1) // remove wrapper
		statefunc.L.Pop(1) // remove function
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": t.OnAfterInsert,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	statefunc.L.SetMetaTable(-2)
	// Now stack: [function, wrapper]
	statefunc.L.Call(1, 0)
}

func (t *Table) runOnAfterUpdate() {
	if syncfunc.GetAfterUpdateRunning() {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	defer syncfunc.SetAfterUpdateRunning(false)
	var tUpdate Table
	tUpdate.Rows = &Rowset{Rows: []Record{}, Pos: 0}
	tUpdate.Rows.Rows = append(tUpdate.Rows.Rows, t.XRecord)

	statefunc.L.Global(t.OnAfterUpdate)
	if !statefunc.L.IsFunction(-1) {
		//fmt.Printf("Lua global '%s' is not a function (type: %v)\n", t.OnAfterUpdate, statefunc.L.TypeOf(-1))
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": t.OnAfterUpdate,
		}), errorhandlefunc.ErrorTypeScript, true)
		statefunc.L.Pop(1)
		return
	}
	// Push the first argument (wrapper)
	wrapper := &TableWrapper{Table: t}
	statefunc.L.PushUserData(wrapper)
	statefunc.L.PushString("TableMT")
	statefunc.L.RawGet(lua.RegistryIndex)
	if statefunc.L.IsNil(-1) {
		statefunc.L.Pop(1)
		statefunc.L.Pop(1) // remove wrapper
		statefunc.L.Pop(1) // remove function
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": t.OnAfterUpdate,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	statefunc.L.SetMetaTable(-2)
	// Push the second argument (wrapper1)
	wrapper1 := &TableWrapper{Table: &tUpdate}
	statefunc.L.PushUserData(wrapper1)
	statefunc.L.PushString("TableMT")
	statefunc.L.RawGet(lua.RegistryIndex)
	if statefunc.L.IsNil(-1) {
		statefunc.L.Pop(1)
		statefunc.L.Pop(1) // remove wrapper1
		statefunc.L.Pop(1) // remove wrapper
		statefunc.L.Pop(1) // remove function
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": t.OnAfterUpdate,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	statefunc.L.SetMetaTable(-2)
	// Now stack: [function, wrapper, wrapper1]
	syncfunc.SetAfterUpdateRunning(true)
	statefunc.L.Call(2, 0)

}

func (t *Table) runOnAfterDelete() {
	defer func() {
		if r := recover(); r != nil {
			errorhandlefunc.ThrowError(r.(string), errorhandlefunc.ErrorTypeScript, true)
		}
	}()
	var tDelete Table
	//tDelete = *t
	tDelete.Rows = &Rowset{Rows: []Record{}, Pos: 0}
	tDelete.Rows.Rows = append(tDelete.Rows.Rows, t.XRecord)
	wrapper := &TableWrapper{Table: &tDelete}
	statefunc.L.Global(t.OnAfterDelete) // Get the function from the Lua global environment
	if !statefunc.L.IsFunction(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.not_a_function", map[string]interface{}{
			"Name": t.OnAfterDelete,
		}), errorhandlefunc.ErrorTypeScript, true)
		return
	}
	statefunc.L.PushUserData(wrapper) // Push the wrapper as userdata
	statefunc.L.PushString("TableMT")
	statefunc.L.RawGet(lua.RegistryIndex)
	if statefunc.L.IsNil(-1) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.tablemt_metatable_not_found", map[string]interface{}{
			"Name": t.OnAfterDelete,
		}), errorhandlefunc.ErrorTypeScript, true)
		//statefunc.L.Pop(1)
		return
	}
	statefunc.L.SetMetaTable(-2)
	statefunc.L.Call(1, 0)
}

// FindByID retrieves a record by ID from the table
func (t *Table) FindByID(id interface{}) bool {
	colStr := "*"
	if len(t.Columns) > 0 {
		var prep []string
		for _, c := range t.Columns {
			prep = append(prep, "\""+c+"\"")
		}
		colStr = strings.Join(prep, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", colStr, t.Name, PrimaryKeyField)
	var rows []Record
	var result = make(Record) //map[string]interface{}
	var r2 = make(map[string]interface{})
	err := t.db.Raw(query, id).Scan(&r2).Error
	if err != nil {
		return false
	}

	if r2 == nil {
		return false
	}
	if len(r2) == 0 {
		return false
	}
	if r2[PrimaryKeyField] == nil {
		return false
	}

	for k, v := range r2 {
		t := fmt.Sprintf("%v", reflect.TypeOf(v))
		if t == "*interface {}" {
			var vp *interface{} = v.(*interface{})
			result[k] = *vp
		} else {
			result[k] = v
		}
	}
	//results = append(results, row)
	if t.Rows == nil {
		rows = append(rows, result)
		t.Rows = &Rowset{Rows: rows, Pos: 0}
		return len(t.Rows.Rows) > 0
	}
	for i, r := range t.Rows.Rows {
		if r[PrimaryKeyField] == id {
			t.Rows.Rows[i] = result
			t.Rows.Pos = i
			return true
		}
	}
	rows = append(rows, result)
	t.Rows = &Rowset{Rows: rows, Pos: 0}
	return len(t.Rows.Rows) > 0
}

// FindByID retrieves a record by ID from the table
func (t *Table) getRecordById(id interface{}) Record {
	colStr := "*"
	statefunc.ClearErrors()
	if len(t.Columns) > 0 {
		var prep []string
		for _, c := range t.Columns {
			prep = append(prep, "\""+c+"\"")
		}
		colStr = strings.Join(prep, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE ID = ?", colStr, t.Name)
	var result = make(map[string]interface{})
	var r2 = make(map[string]interface{})
	tx := t.db.Raw(query, id)
	tx.Take(&r2)
	if tx.Error != nil {
		statefunc.SetLastErrorText(tx.Error.Error())
		return nil
	}
	if r2 == nil {
		return nil
	}
	for k, v := range r2 {
		t := fmt.Sprintf("%v", reflect.TypeOf(v))
		if t == "*interface {}" {
			var vp *interface{} = v.(*interface{})
			result[k] = *vp
		} else {
			result[k] = v
		}
	}
	return result
}

func (t *Table) parseFilterByType(field, filter, fType string) string {
	var r string
	switch fType {
	case typesfunc.TypeDate:
		if filter == "''" {
			return field + " = '' "
		}
		r = timefunc.TemplateToRegexp(timefunc.DateFormat)
		if r == "" {
			return ""
		}
	case typesfunc.TypeTime:
		if filter == "''" {
			return field + " = '' "
		}
		r = timefunc.TemplateToRegexp(timefunc.TimeFormat)
		if r == "" {
			return ""
		}
	case typesfunc.TypeDateTime:
		if filter == "''" {
			return field + " = '' "
		}
		r = timefunc.TemplateToRegexp(timefunc.DateTimeFormat)
		if r == "" {
			return ""
		}
	case typesfunc.TypeBoolean:
		r = `true|false`
	case typesfunc.TypeInteger:
		r = `\d+`
	case typesfunc.TypeReal:
		r = `\d+\.\d+`
	case typesfunc.TypeText:
		if filter == "''" {
			return field + " = '' "
		}
		r = `^[\w\W]+$`
	default:
		return ""
	}
	r0 := `(\&|\|)`
	reg := regexp.MustCompile(r0)
	s := reg.Split(filter, -1)
	var delim []string
	if len(s) == 0 {
		return ""
	}
	if len(s) > 1 {
		delim = reg.FindAllString(filter, -1)
	}
	result := ""
	for i, v := range s {
		var hasRule bool = false
		v = strings.TrimSpace(v)
		if len(v) > 0 {
			switch true {
			case strings.HasPrefix(v, "=="):
				result += field + " = "
				hasRule = true
				v = strings.TrimPrefix(v, "==")
				v = strings.TrimSpace(v)
				// continue
			case strings.HasPrefix(v, "~="):
				result += field + " <> "
				hasRule = true
				v = strings.TrimPrefix(v, "~=")
				v = strings.TrimSpace(v)
				// continue
			case strings.HasPrefix(v, ">"):
				result += field + " > "
				hasRule = true
				v = strings.TrimPrefix(v, ">")
				v = strings.TrimSpace(v)
				// continue
			case strings.HasPrefix(v, "<"):
				result += field + " < "
				hasRule = true
				v = strings.TrimPrefix(v, "<")
				v = strings.TrimSpace(v)
				// continue
			case strings.HasPrefix(v, ">="):
				result += field + " >= "
				hasRule = true
				v = strings.TrimPrefix(v, ">=")
				v = strings.TrimSpace(v)
				// continue
			case strings.HasPrefix(v, "<="):
				result += field + " <= "
				hasRule = true
				v = strings.TrimPrefix(v, "<=")
				v = strings.TrimSpace(v)
			}

			if !hasRule {
				if fType == typesfunc.TypeText && (strings.Contains(v, "%") || strings.Contains(v, "_")) {
					result += field + " LIKE "
				} else {
					result += field + " = "
				}
			}
			switch fType {
			case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime:
				d, err := timefunc.FormatDateTime(v, fType, timefunc.ToInternalFormat)
				if err != nil {
					result += "'" + v + "'"
				} else {
					result += "'" + d + "'"
				}
			case typesfunc.TypeBoolean:
				b, err := boolfunc.FormatBool(v, boolfunc.ToInternalFormat)
				if err != nil {
					result += "'" + v + "'"
				} else {
					result += "'" + b + "'"
				}
			case typesfunc.TypeInteger, typesfunc.TypeReal:
				result += v
			case typesfunc.TypeText:
				v = strings.Trim(v, "'")
				result += "'" + v + "'"
			default:
				result += "'" + v + "'"
			}
			if len(delim) > 0 && i < len(delim) {
				switch strings.ToLower(delim[i]) {
				case "&":
					result += " AND "
				case "|":
					result += " OR "
				}
			}
		}
	}
	return result
}

func (t *Table) parseFilter(field string, filter string) string {
	fType := t.GetFieldType(field)
	if fType == "" {
		return ""
	}
	return t.parseFilterByType(field, filter, fType)
}

// Find retrieves all rows from the table based on current filters and ordering.
//
// This method executes a SELECT query on the table using any filters that have been
// set via SetFilter or SetRangeFilter, and any ordering specified via OrderBy.
//
// Parameters:
//   - None
//
// Returns:
//   - bool: true if rows were found, false otherwise
//
// Example in Lua:
//
//	if table:Find() then
//	  while table:Next() do
//	    print(table:GetField("name"))
//	  end
//	end
func (t *Table) Find() bool {
	statefunc.ClearErrors()
	colStr := "*"
	if len(t.Columns) > 0 {
		var prep []string
		for _, c := range t.Columns {
			prep = append(prep, "\""+c+"\"")
		}
		colStr = strings.Join(prep, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s", colStr, t.Name)
	where := false
	if len(t.plainFilter) > 0 {
		query += " WHERE " + t.plainFilter
		where = true
	} else if len(t.rangeFilter) == 2 {
		query += fmt.Sprintf(" WHERE %s BETWEEN ? AND ?", t.filterByField)
		where = true
	}
	for k, v := range t.filteredFields {
		if len(v) == 0 {
			continue
		}
		f := t.parseFilter(k, v)
		if f == "" {
			continue
		}
		if !where {
			query += " WHERE "
			where = true
		} else {
			query += " AND "
		}
		query += f
	}
	if t.orderBy != "" {
		query += " ORDER BY " + t.orderBy
	}

	var results []Record
	tx := t.db.Raw(query, t.rangeFilter...)
	if tx.Error != nil {
		statefunc.SetLastErrorText(tx.Error.Error())
		return false
	}

	rows, err := tx.Rows()
	if err != nil {
		statefunc.SetLastErrorText(err.Error())
		return false
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		statefunc.SetLastErrorText(err.Error())
		return false
	}
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			statefunc.SetLastErrorText(err.Error())
			return false
		}

		row := make(Record)
		for i, col := range columns {
			val := values[i]
			if val != nil {
				switch v := val.(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = v
				}
			} else {
				row[col] = t.GetDefaultValueForTheField(col)
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		statefunc.SetLastErrorText(err.Error())
		return false
	}

	t.Rows = &Rowset{Rows: results, Pos: 0}
	return len(t.Rows.Rows) > 0
}

func (t *Table) ScrollToBeginning() {
	t.Rows.Pos = 0
}

func (t *Table) ScrollToEnd() {
	t.Rows.Pos = len(t.Rows.Rows) - 1
}

func (t *Table) ScrollToRow(row int) {
	t.Rows.Pos = row
}

// getFieldMetadata retrieves metadata for a specific field
func (t *Table) getFieldMetadata(fieldName string) (*TableMetadata, error) {
	var metadata TableMetadata
	result := t.db.Where("table_name = ? AND field_name = ?", t.Name, fieldName).First(&metadata)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No metadata found
		}
		return nil, result.Error
	}
	return &metadata, nil
}

func (t *Table) GetCurrentRecord() Record {
	if t.Rows == nil || t.Rows.Pos < 0 || t.Rows.Pos >= len(t.Rows.Rows) {
		return nil // No current row
	}
	return t.Rows.Rows[t.Rows.Pos]
}

// GetField gets a field value with type conversion based on metadata
func (t *Table) GetField(field, dtType string) interface{} {
	if t.Rows == nil {
		return nil
	}
	if len(t.Rows.Rows) == 0 || t.Rows.Pos < 0 || t.Rows.Pos >= len(t.Rows.Rows) {
		return t.GetDefaultValueForTheField(field)
	}
	row := t.Rows.Rows[t.Rows.Pos]
	value, exists := row[field]
	if !exists {
		return nil
	}
	switch t.fieldTypes[field] {
	case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime:
		if str, ok := value.(string); ok && str != "" {
			formatted, err := timefunc.FormatDateTime(str, t.fieldTypes[field], timefunc.ToUserFormat)
			if err != nil {
				errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
				return nil
			}
			return formatted
		}
	case typesfunc.TypeBoolean:
		if str, ok := value.(string); ok && str != "" {
			formatted, err := boolfunc.FormatBool(str, boolfunc.ToUserFormat)
			if err != nil {
				errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
				return nil
			}
			return formatted
		}
	case typesfunc.TypeText, typesfunc.TypeInteger, typesfunc.TypeReal:
		if value == nil {
			return t.GetDefaultValueForTheField(field)
		}
	}
	return value
}

func (t *Table) SetField(field string, value interface{}) bool {
	if t.Rows == nil || len(t.Rows.Rows) == 0 || t.Rows.Pos >= len(t.Rows.Rows) {
		t.Init()
	}

	t.Rows.Rows[t.Rows.Pos][field] = value
	return true
}

// SaveField sets a field value with type conversion based on metadata
func (t *Table) SaveField(field string, value interface{}) bool {
	if t.Rows == nil || len(t.Rows.Rows) == 0 {
		return false
	}

	// Add a new row if the current row is the last one
	if t.Rows.Pos >= len(t.Rows.Rows) || (len(t.Rows.Rows) == 1 && t.Rows.Rows[0]["id"] == 0) {
		var fields Record = make(Record)
		var id int64 = 0
		fields[field] = value
		ok := t.Insert(fields, &id)
		if ok && id > 0 {
			syncfunc.BrowseChId = id
		}
		return ok
	}

	// Update the value in the current row
	t.Rows.Rows[t.Rows.Pos][field] = value

	// Update the database
	id := t.Rows.Rows[t.Rows.Pos]["id"].(int64)
	if !t.Update(id, map[string]interface{}{field: value}) {
		errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_update_failed", map[string]interface{}{
			"Field": field,
		}), errorhandlefunc.ErrorTypeScript, true)
		return false
	}

	return true
}

func (t *Table) checkFormatType(value string, extType string) int {
	var rinternal string
	var rcustom string
	switch extType {
	case typesfunc.TypeDate:
		rinternal = timefunc.TemplateToRegexp(timefunc.InternalDateFormat)
		rcustom = timefunc.TemplateToRegexp(timefunc.DateFormat)
	case typesfunc.TypeTime:
		rinternal = timefunc.TemplateToRegexp(timefunc.InternalTimeFormat)
		rcustom = timefunc.TemplateToRegexp(timefunc.TimeFormat)
	case typesfunc.TypeDateTime:
		rinternal = timefunc.TemplateToRegexp(timefunc.InternalDateTimeFormat)
		rcustom = timefunc.TemplateToRegexp(timefunc.DateTimeFormat)
	}
	regexinternal := regexp.MustCompile(rinternal)
	regexcustom := regexp.MustCompile(rcustom)
	if regexinternal.MatchString(value) {
		return 0
	}
	if regexcustom.MatchString(value) {
		return 1
	}

	return -1
}

func (t *Table) fieldUserFormatToInternalFormat(field string, value interface{}, extType string) (any, bool) {
	metadata, _ := t.getFieldMetadata(field)

	var finalValue interface{}
	if metadata != nil {
		switch metadata.LogicalType {
		case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime:
			if str, ok := value.(string); ok {
				if str == "" {
					finalValue = ""
				} else {
					ft := t.checkFormatType(str, metadata.LogicalType)
					if ft == -1 {
						errorhandlefunc.ThrowError(i18nfunc.T("error.db_invalid_type", map[string]interface{}{
							"Type":     metadata.LogicalType,
							"Date":     typesfunc.TypeDate,
							"Time":     typesfunc.TypeTime,
							"DateTime": typesfunc.TypeDateTime,
						}), errorhandlefunc.ErrorTypeScript, true)
						return nil, false
					}
					if ft == 1 {
						formatted, err := timefunc.FormatDateTime(str, metadata.LogicalType, timefunc.ToInternalFormat)
						if err != nil {
							errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
							return nil, false
						}
						finalValue = formatted
					} else {
						finalValue = str
					}
				}
			} else {
				v := *value.(*interface{})
				switch x := v.(type) {
				case string:
					finalValue = x
				case *string:
					finalValue = *x
				default:
					errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_type_mismatch", map[string]interface{}{
						"Field":        field,
						"ExpectedType": "string",
						"ActualType":   reflect.TypeOf(v).String(),
					}), errorhandlefunc.ErrorTypeScript, true)
					return nil, false
				}
			}
			err := timefunc.CheckDateTimeConsistent(finalValue.(string), metadata.LogicalType, timefunc.ToInternalFormat)
			if err != nil {
				errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
				return nil, false
			}
		case typesfunc.TypeBoolean:
			var str string
			switch vt := value.(type) {
			case int, int64:
				str = fmt.Sprintf("%d", vt)
			case string:
				str = vt
			default:
				errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_type_mismatch", map[string]interface{}{
					"Field":        field,
					"ExpectedType": "int, int64, string",
					"ActualType":   reflect.TypeOf(value).String(),
				}), errorhandlefunc.ErrorTypeScript, true)
				return nil, false
			}
			formatted, err := boolfunc.FormatBool(str, boolfunc.ToInternalFormat)
			if err != nil {
				errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
				return nil, false
			}
			finalValue = formatted
		default:
			finalValue = value
		}
	} else if extType != "" {
		if extType != typesfunc.TypeDate && extType != typesfunc.TypeTime && extType != typesfunc.TypeDateTime && extType != "B" {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_invalid_type", map[string]interface{}{
				"Type":     extType,
				"Date":     typesfunc.TypeDate,
				"Time":     typesfunc.TypeTime,
				"DateTime": typesfunc.TypeDateTime,
				"Boolean":  "B",
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil, false
		}
		if str, ok := value.(string); ok {
			if str == "" {
				finalValue = ""
			} else {
				if extType == "B" {
					formatted, err := boolfunc.FormatBool(str, boolfunc.ToInternalFormat)
					if err != nil {
						errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
						return nil, false
					}
					finalValue = formatted
				} else {
					formatted, err := timefunc.FormatDateTime(str, extType, timefunc.ToInternalFormat)
					if err != nil {
						errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeData, false)
						return nil, false
					}
					finalValue = formatted
				}
			}
		} else {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_type_mismatch", map[string]interface{}{
				"Field":        field,
				"ExpectedType": "string",
				"ActualType":   reflect.TypeOf(value).String(),
			}), errorhandlefunc.ErrorTypeScript, true)
			return nil, false
		}
	} else {
		finalValue = value
	}

	return finalValue, true
}

func (t *Table) DeleteRow() bool {
	if t.Rows == nil || t.Rows.Pos < 0 || t.Rows.Pos >= len(t.Rows.Rows) {
		return false //, fmt.Errorf("no current row")
	}
	row := t.Rows.Rows[t.Rows.Pos]
	res := t.delete(row[PrimaryKeyField])
	if res {
		if len(t.Rows.Rows) == 1 {
			t.Rows.Rows = []Record{}
			t.Rows.Pos = 0
		} else {
			t.Rows.Rows = append(t.Rows.Rows[:t.Rows.Pos], t.Rows.Rows[t.Rows.Pos+1:]...)
			t.Rows.Pos--
			if t.Rows.Pos < 0 {
				t.Rows.Pos = 0
			}
		}
	}
	return res
}

// Next moves to the next row and returns it, or nil if at end
func (t *Table) Next() bool {
	if t.Rows.Pos+1 < len(t.Rows.Rows) {
		t.Rows.Pos++
		return true
	}
	return false
}

// Prev moves to the previous row and returns it, or nil if at beginning
func (t *Table) Prev() bool {
	if t.Rows.Pos-1 >= 0 {
		t.Rows.Pos--
		return true
	}
	return false
}

// // Current returns the current row, or nil if out of bounds
//
//	func (r *Rowset) Current() map[string]interface{} {
//		if r.pos >= 0 && r.pos < len(r.rows) {
//			return r.rows[r.pos]
//		}
//		return nil
//	}
//
// AddRow inserts a new row into the table and refreshes the rowset
func (t *Table) AddRow(field string, value interface{}) bool {
	var id int64
	fields := make(map[string]interface{})
	// Set all fields to their default values
	for _, col := range t.Columns {
		fields[col] = t.defaultFieldValues[col]
	}
	// Set the provided field to the given value
	if value != nil {
		fields[field] = value
	}
	success := t.Insert(fields, &id)
	if !success {
		return false
	}
	return t.Find()
}

// GetDefaultValueForTheField returns the default value for the specified field (column) name
// in the table. It queries the database schema using PRAGMA table_info to determine the default
// value for each column. If the column has an explicit default value, it is used; otherwise,
// a type-appropriate zero value is assigned (e.g., 0 for integers, "" for strings, etc.).
// The function returns the default value for the requested field, or nil/false if the field
// does not exist or an error occurs during schema inspection.
func (t *Table) GetDefaultValueForTheField(field string) interface{} {
	if t.defaultFieldValues == nil {
		if !t.fillFieldsMeta() {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_scan_failed", nil), errorhandlefunc.ErrorTypeScript, true)
			return nil
		}
	}
	return t.defaultFieldValues[field]
}

func (t *Table) GetFieldType(field string) string {
	if t.fieldTypes == nil {
		if !t.fillFieldsMeta() {
			errorhandlefunc.ThrowError(i18nfunc.T("error.db_field_scan_failed", nil), errorhandlefunc.ErrorTypeScript, true)
			return ""
		}
	}
	return t.fieldTypes[field]
}

func (t *Table) fillFieldsMeta() bool {
	// Get column types from PRAGMA table_info
	rows, err := t.db.Raw("PRAGMA table_info(" + t.Name + ")").Rows()
	if err != nil {
		return false
	}
	defer rows.Close()
	//colDefaults := make(map[string]interface{})
	for rows.Next() {
		var cid int
		var colName, colType string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		metadata, _ := t.getFieldMetadata(colName)
		if metadata != nil {
			// If metadata exists, use its default value if available
			var tp string
			if metadata.LogicalType != "" {
				tp = metadata.LogicalType
			} else {
				tp = metadata.ActualType
			}
			t.fieldTypes[colName] = tp
			switch tp {
			case typesfunc.TypeDate, typesfunc.TypeTime, typesfunc.TypeDateTime, "TEXT":
				t.defaultFieldValues[colName] = ""
			case typesfunc.TypeBoolean:
				t.defaultFieldValues[colName] = 0
			case typesfunc.TypeInteger, "INT":
				t.defaultFieldValues[colName] = 0
			case typesfunc.TypeReal, "FLOAT", "DOUBLE":
				t.defaultFieldValues[colName] = 0.0
			}
		} else {
			// Set default value based on type if dfltValue is nil
			if dfltValue != nil {
				t.defaultFieldValues[colName] = dfltValue
			} else {
				switch {
				case strings.HasPrefix(colType, "INT"):
					t.defaultFieldValues[colName] = 0
					t.fieldTypes[colName] = typesfunc.TypeInteger
				case strings.HasPrefix(colType, "REAL"), strings.HasPrefix(colType, "FLOAT"), strings.HasPrefix(colType, "DOUBLE"):
					t.defaultFieldValues[colName] = 0.0
					t.fieldTypes[colName] = typesfunc.TypeReal
				case strings.HasPrefix(colType, "TEXT"), strings.HasPrefix(colType, "CHAR"), strings.HasPrefix(colType, "VARCHAR"):
					t.defaultFieldValues[colName] = ""
					t.fieldTypes[colName] = "TEXT"
				case strings.HasPrefix(colType, "BLOB"):
					t.defaultFieldValues[colName] = []byte{}
					t.fieldTypes[colName] = "BLOB"
				default:
					t.defaultFieldValues[colName] = nil
					t.fieldTypes[colName] = "NULL"
				}
			}
		}
	}
	return true

}

func (t *Table) Init() {
	if t.Rows == nil {
		if !t.Find() {
			t.Rows = &Rowset{}
			t.Rows.Rows = []Record{}
			t.Rows.Rows = append(t.Rows.Rows, map[string]interface{}{})
			t.Rows.Pos = 0
		} else {
			t.Rows.Rows = append(t.Rows.Rows, map[string]interface{}{})
			t.Rows.Pos = len(t.Rows.Rows) - 1
		}
	} else {
		t.Rows.Rows = append(t.Rows.Rows, map[string]interface{}{})
		t.Rows.Pos = len(t.Rows.Rows) - 1
	}
	fields := &t.Rows.Rows[t.Rows.Pos]

	//Get column types from PRAGMA table_info
	rows, err := t.db.Raw("PRAGMA table_info(" + t.Name + ")").Rows()
	if err != nil {
		return
	}
	defer rows.Close()
	//colDefaults := &t.Rows.Rows[t.Rows.Pos] //make(map[string]interface{})
	for rows.Next() {
		var cid int
		var colName, colType string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dfltValue, &pk); err != nil {
			return
		}
		(*fields)[colName] = t.defaultFieldValues[colName]
	}

}

// DropTable drops a table and its metadata from the database
func DropTable(db *gorm.DB, name string) error {
	// Delete any metadata associated with this table, ignore if none exists
	db.Where("table_name = ?", name).Delete(&TableMetadata{})
	// Note: We intentionally ignore the result here as it's okay if no metadata exists

	// Drop the actual table
	result := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	if result.Error != nil {
		return errors.New(i18nfunc.T("error.db_table_drop_failed", map[string]interface{}{
			"Name": name,
		}))
	}

	return nil
}

// tableExists checks if a table exists in the database
func tableExists(db *gorm.DB, tableName string) bool {
	// For SQLite, we can check sqlite_master table
	var count int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Count(&count)
	return count > 0
}
