package syncfunc

var BrowseChId int64
var afterUpdateRunning bool
var afterInsertRunning bool
var afterDeleteRunning bool
var lookupSuccess bool

func SetAfterUpdateRunning(r bool) {
	afterUpdateRunning = r
}

func SetAfterInsertRunning(r bool) {
	afterInsertRunning = r
}

func SetAfterDeleteRunning(r bool) {
	afterDeleteRunning = r
}

func GetAfterUpdateRunning() bool {
	return afterUpdateRunning
}

func GetAfterInsertRunning() bool {
	return afterInsertRunning
}

func GetAfterDeleteRunning() bool {
	return afterDeleteRunning
}

func SetLookupSuccess(r bool) {
	lookupSuccess = r
}

func GetLookupSuccess() bool {
	return lookupSuccess
}
