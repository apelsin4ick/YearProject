package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Конфигурация
const (
	WeatherAPIKey = "38c3540b855e40cf8c2150413251505"                
	BotToken      = "7889032903:AAGcqqPXteJxifA4B3cYrV1706nz88fYPVA"
)

// Структуры для парсинга JSON от WeatherAPI
type WeatherResponse struct {
	Location Location `json:"location"`
	Current  Current  `json:"current"`
	Forecast Forecast `json:"forecast"`
}

type Location struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}

type Current struct {
	TempC      float64 `json:"temp_c"`
	FeelsLikeC float64 `json:"feelslike_c"`
	Condition  struct {
		Text string `json:"text"`
	} `json:"condition"`
	WindKph  float64 `json:"wind_kph"`
	Humidity int     `json:"humidity"`
}

type Forecast struct {
	ForecastDay []ForecastDay `json:"forecastday"`
}

type ForecastDay struct {
	Date string `json:"date"`
	Day  struct {
		MaxTempC  float64 `json:"maxtemp_c"`
		MinTempC  float64 `json:"mintemp_c"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
	} `json:"day"`
}

// UserData хранит данные пользователя
type UserData struct {
	RecentCities []string `json:"recent_cities"`
	Action       string   `json:"action"`
}

// Глобальная переменная для хранения данных пользователей
var userDataMap = make(map[int64]*UserData)

// Получение основной клавиатуры
func getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Погода сейчас"),
			tgbotapi.NewKeyboardButton("Прогноз на 3 дня"),
		),
	)
}

// Получение клавиатуры с городами
func getCitiesKeyboard(cities []string) tgbotapi.ReplyKeyboardMarkup {
	var rows [][]tgbotapi.KeyboardButton
	for _, city := range cities {
		rows = append(rows, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(city)))
	}
	rows = append(rows, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Назад"),
	))
	return tgbotapi.NewReplyKeyboard(rows...)
}

// Запрос погоды
func getWeather(city string, days int) (*WeatherResponse, error) {
	baseURL := "http://api.weatherapi.com/v1/forecast.json"

	params := url.Values{}
	params.Add("key", WeatherAPIKey)
	params.Add("q", city)
	params.Add("days", fmt.Sprintf("%d", days))
	params.Add("lang", "ru")

	resp, err := http.Get(fmt.Sprintf("%s?%s", baseURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var weatherData WeatherResponse
	err = json.NewDecoder(resp.Body).Decode(&weatherData)
	if err != nil {
		return nil, err
	}

	return &weatherData, nil
}

// Обработчик /start
func handleStart(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Привет! Я бот погоды. Напиши название города или выбери действие:")
	msg.ReplyMarkup = getMainKeyboard()
	bot.Send(msg)
}

// Обработчик сообщений
func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	text := update.Message.Text
	chatID := update.Message.Chat.ID

	// Инициализация данных пользователя, если их нет
	if _, ok := userDataMap[chatID]; !ok {
		userDataMap[chatID] = &UserData{
			RecentCities: []string{},
		}
	}
	userData := userDataMap[chatID]

	switch text {
	case "Погода сейчас":
		if len(userData.RecentCities) > 0 {
			msg := tgbotapi.NewMessage(chatID, "Выбери город или напиши новый:")
			msg.ReplyMarkup = getCitiesKeyboard(userData.RecentCities)
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "Напиши название города:")
			bot.Send(msg)
		}
		userData.Action = "current"

	case "Прогноз на 3 дня":
		if len(userData.RecentCities) > 0 {
			msg := tgbotapi.NewMessage(chatID, "Выбери город или напиши новый:")
			msg.ReplyMarkup = getCitiesKeyboard(userData.RecentCities)
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "Напиши название города:")
			bot.Send(msg)
		}
		userData.Action = "forecast"

	case "Назад":
		msg := tgbotapi.NewMessage(chatID, "Выбери действие:")
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)

	default:
		city := text
		action := userData.Action

		// Обновляем список последних городов (максимум 3)
		if !contains(userData.RecentCities, city) {
			userData.RecentCities = append([]string{city}, userData.RecentCities...)
			if len(userData.RecentCities) > 3 {
				userData.RecentCities = userData.RecentCities[:3]
			}
		}

		switch action {
		case "current":
			weatherData, err := getWeather(city, 1)
			if err != nil {
				msg := tgbotapi.NewMessage(chatID, "Ошибка при получении погоды. Попробуйте еще раз.")
				bot.Send(msg)
				return
			}

			message := fmt.Sprintf(
				"🌤 Погода в %s (%s):\n"+
					"🌡 Температура: %.1f°C (ощущается как %.1f°C)\n"+
					"☁ Состояние: %s\n"+
					"💨 Ветер: %.1f км/ч\n"+
					"💧 Влажность: %d%%",
				weatherData.Location.Name,
				weatherData.Location.Country,
				weatherData.Current.TempC,
				weatherData.Current.FeelsLikeC,
				weatherData.Current.Condition.Text,
				weatherData.Current.WindKph,
				weatherData.Current.Humidity,
			)
			msg := tgbotapi.NewMessage(chatID, message)
			msg.ReplyMarkup = getMainKeyboard()
			bot.Send(msg)

		case "forecast":
			weatherData, err := getWeather(city, 3)
			if err != nil {
				msg := tgbotapi.NewMessage(chatID, "Ошибка при получении прогноза. Попробуйте еще раз.")
				bot.Send(msg)
				return
			}

			message := fmt.Sprintf("📆 Прогноз на 3 дня для %s:\n\n", weatherData.Location.Name)
			for _, day := range weatherData.Forecast.ForecastDay {
				message += fmt.Sprintf(
					"📅 %s:\n"+
						"⬆ Макс: %.1f°C, ⬇ Мин: %.1f°C\n"+
						"☁ %s\n\n",
					day.Date,
					day.Day.MaxTempC,
					day.Day.MinTempC,
					day.Day.Condition.Text,
				)
			}
			msg := tgbotapi.NewMessage(chatID, message)
			msg.ReplyMarkup = getMainKeyboard()
			bot.Send(msg)

		default:
			msg := tgbotapi.NewMessage(chatID, "Выбери действие из меню 👇")
			msg.ReplyMarkup = getMainKeyboard()
			bot.Send(msg)
		}
	}
}

// Вспомогательная функция для проверки наличия элемента в слайсе
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func main() {
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				handleStart(bot, update)
			}
		} else {
			handleMessage(bot, update)
		}
	}
}
