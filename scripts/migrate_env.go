package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func flattenMap(prefix string, m map[string]interface{}) map[string]string {
	flatMap := make(map[string]string)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		key = strings.ToUpper(key)

		switch val := v.(type) {
		case map[string]interface{}:
			nested := flattenMap(key, val)
			for nk, nv := range nested {
				flatMap[nk] = nv
			}
		case string:
			flatMap[key] = fmt.Sprintf("%q", val)
		default:
			flatMap[key] = fmt.Sprintf("%v", val)
		}
	}
	return flatMap
}

func main() {
	// Read YAML file
	yamlData, err := os.ReadFile("config/config.yaml")
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// Flatten the map
	envVars := flattenMap("", config)

	// Create .env content
	var envContent strings.Builder
	for k, v := range envVars {
		// If the value is already quoted (string), use it as is
		// Otherwise, it's a number or boolean, so use it without quotes
		if strings.HasPrefix(v, `"`) {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		} else {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	// Write to .env file
	if err := os.WriteFile(".env", []byte(envContent.String()), 0o644); err != nil {
		fmt.Printf("Error writing .env file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully converted config.yaml to .env file!")
}
