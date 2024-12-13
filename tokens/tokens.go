package tokens

import (
	"encoding/json"
	"os"
)

// Структура для хранения токенов
type Config struct {
	Tokens []string `json:"tokens"`
}

// Загрузка токенов из файла
func LoadTokensFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}

	return config.Tokens, nil
}
