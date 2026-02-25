package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// getLangOverrideDir returns the config-based directory for translation overrides
func getLangOverrideDir() string {
	return System.Folder.Config + "lang" + string(os.PathSeparator)
}

// getTranslations reads all language files and returns them as a map
func getTranslations() (translations map[string]map[string]interface{}, langs []string, err error) {
	translations = make(map[string]map[string]interface{})
	langs = make([]string, 0)

	langDir := "html/lang/"
	overrideDir := getLangOverrideDir()

	if System.Dev {
		// In dev mode, read from filesystem
		files, readErr := os.ReadDir(langDir)
		if readErr != nil {
			err = readErr
			return
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".json") {
				lang := strings.TrimSuffix(f.Name(), ".json")
				langs = append(langs, lang)
				langMap, loadErr := loadJSONFileToMap(langDir + f.Name())
				if loadErr != nil {
					showWarning(2099)
					continue
				}
				translations[lang] = langMap
			}
		}
	} else {
		// In production, read from embedded webUI
		for key, value := range webUI {
			if strings.HasPrefix(key, "html/lang/") && strings.HasSuffix(key, ".json") {
				lang := strings.TrimSuffix(filepath.Base(key), ".json")
				langs = append(langs, lang)
				content := GetHTMLString(value.(string))
				langMap := jsonToMap(content)
				translations[lang] = langMap
			}
		}
	}

	// Check for override files in config directory (both dev and production)
	if _, statErr := os.Stat(overrideDir); statErr == nil {
		files, readErr := os.ReadDir(overrideDir)
		if readErr == nil {
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") {
					lang := strings.TrimSuffix(f.Name(), ".json")
					langMap, loadErr := loadJSONFileToMap(overrideDir + f.Name())
					if loadErr != nil {
						continue
					}
					translations[lang] = langMap
					// Add to langs list if not already there
					found := false
					for _, l := range langs {
						if l == lang {
							found = true
							break
						}
					}
					if !found {
						langs = append(langs, lang)
					}
				}
			}
		}
	}

	return
}

// saveTranslation saves a translation map to a language file in config directory
func saveTranslation(lang string, translations map[string]interface{}) error {
	if lang == "" {
		return errors.New("language code is required")
	}

	// Validate language code (only alphanumeric and hyphens)
	validLang := regexp.MustCompile(`^[a-zA-Z]{2}(-[a-zA-Z]{2})?$`)
	if !validLang.MatchString(lang) {
		return errors.New("invalid language code")
	}

	overrideDir := getLangOverrideDir()
	// Ensure directory exists
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		return err
	}

	file := overrideDir + lang + ".json"

	// Pretty-print JSON
	jsonData, err := json.MarshalIndent(translations, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(file, jsonData, 0644)
	if err != nil {
		return err
	}

	showInfo(fmt.Sprintf("Translations:Saved language file: %s", file))
	return nil
}

// addLanguage creates a new language file based on English template
func addLanguage(langCode string) error {
	if langCode == "" {
		return errors.New("language code is required")
	}

	// Validate language code
	validLang := regexp.MustCompile(`^[a-zA-Z]{2}(-[a-zA-Z]{2})?$`)
	if !validLang.MatchString(langCode) {
		return errors.New("invalid language code format (use: xx or xx-XX)")
	}

	overrideDir := getLangOverrideDir()
	newFile := overrideDir + langCode + ".json"

	// Check if file already exists
	if _, err := os.Stat(newFile); err == nil {
		return fmt.Errorf("language file already exists: %s", langCode)
	}

	// Load English as template
	var enMap map[string]interface{}
	var err error

	if System.Dev {
		enMap, err = loadJSONFileToMap("html/lang/en.json")
	} else {
		enFile := "html/lang/en.json"
		if value, ok := webUI[enFile]; ok {
			content := GetHTMLString(value.(string))
			enMap = jsonToMap(content)
		} else {
			return errors.New("english template not found")
		}
	}

	if err != nil {
		return err
	}

	// Ensure directory exists
	if mkErr := os.MkdirAll(overrideDir, 0755); mkErr != nil {
		return mkErr
	}

	// Save as new language file
	jsonData, err := json.MarshalIndent(enMap, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(newFile, jsonData, 0644)
	if err != nil {
		return err
	}

	showInfo(fmt.Sprintf("Translations:Created new language file: %s", newFile))
	return nil
}
