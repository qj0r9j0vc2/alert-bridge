package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GetAlertFixture loads an alert fixture by name
func GetAlertFixture(name string) Alert {
	fixturesPath := filepath.Join("test", "e2e", "fixtures", "alerts.json")

	data, err := os.ReadFile(fixturesPath)
	if err != nil {
		panic("Failed to read alerts.json: " + err.Error())
	}

	var fixtures map[string]Alert
	if err := json.Unmarshal(data, &fixtures); err != nil {
		panic("Failed to parse alerts.json: " + err.Error())
	}

	fixture, ok := fixtures[name]
	if !ok {
		panic("Fixture not found: " + name)
	}

	return fixture
}

// GetAllAlertFixtures loads all alert fixtures
func GetAllAlertFixtures() map[string]Alert {
	fixturesPath := filepath.Join("test", "e2e", "fixtures", "alerts.json")

	data, err := os.ReadFile(fixturesPath)
	if err != nil {
		panic("Failed to read alerts.json: " + err.Error())
	}

	var fixtures map[string]Alert
	if err := json.Unmarshal(data, &fixtures); err != nil {
		panic("Failed to parse alerts.json: " + err.Error())
	}

	return fixtures
}
