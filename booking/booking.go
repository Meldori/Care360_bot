package booking

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// DoctorCategory представляет категорию врача
type DoctorCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// DoctorAvailability содержит данные о доступных датах и времени врача
type DoctorAvailability struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

// FetchDoctorCategories получает список категорий врачей из API
func FetchDoctorCategories(apiURL string) ([]DoctorCategory, error) {
	resp, err := http.Get(apiURL + "/categories")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var categories []DoctorCategory
	err = json.Unmarshal(body, &categories)
	if err != nil {
		return nil, err
	}

	return categories, nil
}

// FetchDoctorAvailability получает доступное время врача по его категории
func FetchDoctorAvailability(apiURL string, categoryID int) ([]DoctorAvailability, error) {
	resp, err := http.Get(fmt.Sprintf("%s/availability?category_id=%d", apiURL, categoryID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var availability []DoctorAvailability
	err = json.Unmarshal(body, &availability)
	if err != nil {
		return nil, err
	}

	return availability, nil
}

// FetchTimeSlots получает список временных слотов для выбранной даты
func FetchTimeSlots(apiURL string, selectedDate string) ([]string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/timeslots?date=%s", apiURL, selectedDate))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var timeSlots []string
	err = json.Unmarshal(body, &timeSlots)
	if err != nil {
		return nil, err
	}

	return timeSlots, nil
}

// HandleBooking обновлен для обработки категорий и доступности
func HandleBooking(bot *tgbotapi.BotAPI, update *tgbotapi.Update, apiURL string) {
	data := update.CallbackQuery.Data

	switch {
	case data == "select_category":
		categories, err := FetchDoctorCategories(apiURL)
		if err != nil {
			log.Printf("Ошибка загрузки категорий: %v", err)
			bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка загрузки категорий врачей."))
			return
		}

		var rows [][]tgbotapi.InlineKeyboardButton
		for _, category := range categories {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(category.Name, fmt.Sprintf("category:%d", category.ID)),
			))
		}

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Выберите категорию врача:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		bot.Send(msg)

	case len(data) > 9 && data[:9] == "category:":
		var categoryID int
		fmt.Sscanf(data[9:], "%d", &categoryID)
		availability, err := FetchDoctorAvailability(apiURL, categoryID)
		if err != nil {
			log.Printf("Ошибка загрузки доступности: %v", err)
			bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка загрузки доступного времени."))
			return
		}

		var rows [][]tgbotapi.InlineKeyboardButton
		for _, slot := range availability {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s", slot.Date), fmt.Sprintf("date:%s", slot.Date)),
			))
		}

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Выберите дату приёма:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		bot.Send(msg)

	case len(data) > 5 && data[:5] == "date:":
		selectedDate := data[5:]
		timeSlots, err := FetchTimeSlots(apiURL, selectedDate)
		if err != nil {
			log.Printf("Ошибка загрузки временных слотов: %v", err)
			bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка загрузки времени для выбранной даты."))
			return
		}

		var rows [][]tgbotapi.InlineKeyboardButton
		for _, time := range timeSlots {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(time, fmt.Sprintf("time:%s", time)),
			))
		}

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, fmt.Sprintf("Доступное время для %s:", selectedDate))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		bot.Send(msg)

	default:
		log.Printf("Некорректные данные CallbackQuery: %s", data)
		bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка: действие не распознано. Попробуйте снова."))
	}
}
