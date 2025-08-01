package i18nfunc

import (
	"embed"
)

//go:embed *.json
var embeddedTranslations embed.FS
