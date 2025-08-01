package i18nfunc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle
var localizer *i18n.Localizer

// InitI18n initializes the i18n system with the given default language
func InitI18n(defaultLang string) error {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// First try to load external translations
	if err := loadExternalTranslations(); err != nil {
		// If external translations fail, try embedded translations
		if err := loadEmbeddedTranslations(); err != nil {
			return fmt.Errorf("failed to load any translations: %v", err)
		}
	}

	// Set the localizer with the default language
	setLanguage(defaultLang)
	return nil
}

// loadExternalTranslations loads translations from the external translations directory
func loadExternalTranslations() error {
	translationsDir := "translations"
	files, err := os.ReadDir(translationsDir)
	if err != nil {
		return fmt.Errorf("failed to read translations directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			_, err := bundle.LoadMessageFile(filepath.Join(translationsDir, file.Name()))
			if err != nil {
				return fmt.Errorf("failed to load translation file %s: %v", file.Name(), err)
			}
		}
	}
	return nil
}

// loadEmbeddedTranslations loads translations from embedded files
func loadEmbeddedTranslations() error {
	entries, err := embeddedTranslations.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded translations: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := embeddedTranslations.ReadFile(entry.Name())
			if err != nil {
				return fmt.Errorf("failed to read embedded translation file %s: %v", entry.Name(), err)
			}

			_, err = bundle.ParseMessageFileBytes(data, entry.Name())
			if err != nil {
				return fmt.Errorf("failed to parse embedded translation file %s: %v", entry.Name(), err)
			}
		}
	}
	return nil
}

// setLanguage changes the current language
func setLanguage(lang string) {
	localizer = i18n.NewLocalizer(bundle, lang)
}

// T translates a message ID to the current language
func T(messageID string, templateData map[string]interface{}) string {
	if localizer == nil {
		return messageID
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID
	}
	return msg
}
