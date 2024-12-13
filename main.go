package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"Care360/tokens"
	"Care360/userdata"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var bookedTimes = make(map[string]bool)

// DoctorCategory represents a category of doctors
type DoctorCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type DoctorData struct {
	Doctors []struct {
		UserID     int    `json:"user_id"`
		ID         int    `json:"id"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		SecondName string `json:"second_name"`
		Profession string `json:"profession"`
	} `json:"doctors"`
	Branches []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		City    string `json:"city"`
		Address string `json:"address"`
	} `json:"branches"`
	DoctorTimes []struct {
		Date      string `json:"date"`
		TimeBegin string `json:"time_begin"`
		TimeEnd   string `json:"time_end"`
		DoctorID  int    `json:"doctor_id"`
		BranchID  int    `json:"branch_id"`
	} `json:"doctor_times"`
}

func fetchDoctorData(apiURL string) (*DoctorData, error) {
	reqURL := apiURL + "/global/doctor_time"
	log.Printf("Запрос к API: %s", reqURL)
	resp, err := http.Get(reqURL)
	if err != nil {
		log.Printf("Ошибка запроса к API: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Ошибка чтения ответа API: %v", err)
		return nil, err
	}

	log.Printf("Ответ API: %s", string(body))

	var data DoctorData
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Printf("Ошибка разбора JSON: %v", err)
		return nil, err
	}

	return &data, nil
}

func startBot(botToken string, apiURL string, wg *sync.WaitGroup) {
	defer wg.Done()

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Printf("Ошибка при запуске бота с токеном %s: %v", botToken, err)
		return
	}

	bot.Debug = true
	log.Printf("Авторизован как %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message", "callback_query"}

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			switch update.Message.Text {
			case "/start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать! Выберите действие:")
				msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButton("Записаться на приём"),
						tgbotapi.NewKeyboardButton("Информация о клинике"),
					),
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButton("Личный кабинет"),
					),
				)
				bot.Send(msg)
			case "Записаться на приём":
				data, err := fetchDoctorData(apiURL)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка загрузки данных категорий. Пожалуйста, попробуйте позже."))
					log.Printf("Ошибка загрузки данных: %v", err)
					continue
				}

				categories := map[string]bool{}
				for _, doctor := range data.Doctors {
					categories[doctor.Profession] = true
				}

				var rows [][]tgbotapi.InlineKeyboardButton
				for category := range categories {
					callbackData := fmt.Sprintf("category:%s", category)
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(category, callbackData),
					))
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите категорию врачей:")
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
				bot.Send(msg)
			case "Информация о клинике":
				data, err := fetchDoctorData(apiURL)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка загрузки информации о филиалах."))
					continue
				}

				if len(data.Branches) == 0 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Филиалы клиники временно недоступны."))
					continue
				}

				response := "Филиалы клиники:\n"
				for _, branch := range data.Branches {
					response += fmt.Sprintf("%s, %s, %s\n", branch.Name, branch.City, branch.Address)
				}
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, response))
			case "Личный кабинет":
				userID := int(update.Message.From.ID)
				users, err := userdata.LoadUserData("name.json")
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка загрузки данных пользователей."))
					continue
				}
				user, err := userdata.GetUserDataByID(users, userID)
				if err == nil {
					response := fmt.Sprintf("Ваши данные:\nФИО: %s\nНомер: %s\nID: %d", user.Name, user.Phone, user.ID)
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, response))
				} else {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ваши данные не найдены."))
				}
			}
		} else if update.CallbackQuery != nil {
			callbackData := update.CallbackQuery.Data
			if callbackData != "" {
				splitData := strings.Split(callbackData, ":")
				switch splitData[0] {
				case "category":
					category := splitData[1]
					data, err := fetchDoctorData(apiURL)
					if err != nil {
						bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка загрузки данных о датах."))
						continue
					}

					availableDates := map[string]bool{}
					for _, time := range data.DoctorTimes {
						availableDates[time.Date] = true
					}

					var rows [][]tgbotapi.InlineKeyboardButton
					for date := range availableDates {
						callbackData := fmt.Sprintf("date:%s:%s", category, date)
						rows = append(rows, tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData(date, callbackData),
						))
					}

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Выберите дату:")
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
					bot.Send(msg)
				case "date":
					category := splitData[1]
					date := splitData[2]
					data, err := fetchDoctorData(apiURL)
					if err != nil {
						bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка загрузки данных о времени."))
						continue
					}

					timeSlots := []string{}
					for _, time := range data.DoctorTimes {
						if time.Date == date {
							timeSlots = append(timeSlots, fmt.Sprintf("%s - %s", time.TimeBegin, time.TimeEnd))
						}
					}

					var rows [][]tgbotapi.InlineKeyboardButton
					for _, slot := range timeSlots {
						callbackData := fmt.Sprintf("time:%s:%s:%s", category, date, slot)
						rows = append(rows, tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData(slot, callbackData),
						))
					}

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Выберите время:")
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
					bot.Send(msg)
				case "time":
					category := splitData[1]
					date := splitData[2]
					time := splitData[3]
					confirmation := fmt.Sprintf("Вы выбрали категорию: %s, дату: %s, время: %s", category, date, time)
					bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, confirmation))
				}
			}
		}
	}
}

func main() {
	tokens, err := tokens.LoadTokensFromFile("tokens.json")
	if err != nil {
		log.Fatalf("Ошибка загрузки токенов: %v", err)
	}

	const apiURL = "https://app.future-it-pro.ru/api"

	var wg sync.WaitGroup

	for _, token := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			startBot(token, apiURL, &wg)
		}(token)
	}

	wg.Wait()
}
