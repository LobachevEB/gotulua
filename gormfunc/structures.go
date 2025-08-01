package gormfunc

type Record map[string]interface{}

// Rowset is a helper for iterating rows forward and backward
type Rowset struct {
	Rows []Record // Each row is a map of field names to values
	Pos  int
}
