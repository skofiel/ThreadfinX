package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// getAvailableLangCodes returns just the list of available language codes (lightweight, no content loading)
func getAvailableLangCodes() []string {
	langSet := make(map[string]bool)

	langDir := "html/lang/"
	if System.Dev {
		files, err := os.ReadDir(langDir)
		if err == nil {
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") {
					langSet[strings.TrimSuffix(f.Name(), ".json")] = true
				}
			}
		}
	} else {
		for key := range webUI {
			if strings.HasPrefix(key, "html/lang/") && strings.HasSuffix(key, ".json") {
				langSet[strings.TrimSuffix(filepath.Base(key), ".json")] = true
			}
		}
	}

	// Check override directory
	overrideDir := getLangOverrideDir()
	if _, err := os.Stat(overrideDir); err == nil {
		files, err := os.ReadDir(overrideDir)
		if err == nil {
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") {
					langSet[strings.TrimSuffix(f.Name(), ".json")] = true
				}
			}
		}
	}

	langs := make([]string, 0, len(langSet))
	for lang := range langSet {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// deleteLanguage removes a custom language file from the config override directory
func deleteLanguage(langCode string) error {
	if langCode == "" {
		return errors.New("language code is required")
	}

	// Never allow deleting English - it's the base language
	if langCode == "en" {
		return errors.New("cannot delete the default language (en)")
	}

	// Validate language code
	validLang := regexp.MustCompile(`^[a-zA-Z]{2}(-[a-zA-Z]{2})?$`)
	if !validLang.MatchString(langCode) {
		return errors.New("invalid language code")
	}

	overrideDir := getLangOverrideDir()
	file := overrideDir + langCode + ".json"

	// Check if override file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// Check if it's a built-in language (can't delete those)
		isBuiltin := false
		if System.Dev {
			if _, statErr := os.Stat("html/lang/" + langCode + ".json"); statErr == nil {
				isBuiltin = true
			}
		} else {
			if _, ok := webUI["html/lang/"+langCode+".json"]; ok {
				isBuiltin = true
			}
		}
		if isBuiltin {
			return fmt.Errorf("cannot delete built-in language: %s", langCode)
		}
		return fmt.Errorf("language file not found: %s", langCode)
	}

	// Delete the override file
	if err := os.Remove(file); err != nil {
		return err
	}

	// If the current language was deleted, fall back to English
	if Settings.Language == langCode {
		Settings.Language = "en"
		saveSettings(Settings)
	}

	showInfo(fmt.Sprintf("Translations:Deleted language file: %s", file))
	return nil
}
