package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

//  хранит данные пользователя
type UserData struct {
	RecentCities []string `json:"recent_cities"`
	Action       string   `json:"action"`
}

//  хранит статистику производительности
type PerformanceStats struct {
	mu                  sync.Mutex
	TotalRequests       int64
	TotalResponseTime   time.Duration
	Last10ResponseTimes []time.Duration
	Errors              int64
	APIResponseTimes    []APIResponseTime
}

type APIResponseTime struct {
	Endpoint  string
	Duration  time.Duration
	Timestamp time.Time
}

// Глобальные переменные
var (
	userDataMap = make(map[int64]*UserData)
	stats       PerformanceStats
)

// Update обновляет статистику производительности
func (ps *PerformanceStats) Update(duration time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.TotalRequests++
	ps.TotalResponseTime += duration

	ps.Last10ResponseTimes = append(ps.Last10ResponseTimes, duration)
	if len(ps.Last10ResponseTimes) > 10 {
		ps.Last10ResponseTimes = ps.Last10ResponseTimes[1:]
	}
}

// записывает время ответа API
func (ps *PerformanceStats) RecordAPIResponse(endpoint string, duration time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.APIResponseTimes = append(ps.APIResponseTimes, APIResponseTime{
		Endpoint:  endpoint,
		Duration:  duration,
		Timestamp: time.Now(),
	})

	if len(ps.APIResponseTimes) > 100 {
		ps.APIResponseTimes = ps.APIResponseTimes[1:]
	}
}

// увеличивает счетчик ошибок
func (ps *PerformanceStats) IncrementErrors() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Errors++
}

//  возвращает статистику в виде строки
func (ps *PerformanceStats) String() string {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	avgResponse := time.Duration(0)
	if ps.TotalRequests > 0 {
		avgResponse = ps.TotalResponseTime / time.Duration(ps.TotalRequests)
	}

	last10Avg := time.Duration(0)
	if len(ps.Last10ResponseTimes) > 0 {
		var sum time.Duration
		for _, t := range ps.Last10ResponseTimes {
			sum += t
		}
		last10Avg = sum / time.Duration(len(ps.Last10ResponseTimes))
	}

	lastAPI := "нет данных"
	if len(ps.APIResponseTimes) > 0 {
		last := ps.APIResponseTimes[len(ps.APIResponseTimes)-1]
		lastAPI = fmt.Sprintf("%s (%.2f ms)", last.Endpoint, last.Duration.Seconds()*1000)
	}

	return fmt.Sprintf(
		"📊 Статистика производительности:\n\n"+
			"• Всего запросов: %d\n"+
			"• Среднее время обработки: %.2f ms\n"+
			"• Среднее время (последние 10): %.2f ms\n"+
			"• Ошибок: %d\n"+
			"• Последний запрос к API: %s",
		ps.TotalRequests,
		avgResponse.Seconds()*1000,
		last10Avg.Seconds()*1000,
		ps.Errors,
		lastAPI,
	)
}

// Получение основной клавиатуры
func getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Погода сейчас"),
			tgbotapi.NewKeyboardButton("Прогноз на 3 дня"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Статистика скорости"),
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

// Запрос погоды с замером времени
func getWeather(city string, days int) (*WeatherResponse, error) {
	start := time.Now()

	baseURL := "http://api.weatherapi.com/v1/forecast.json"
	params := url.Values{}
	params.Add("key", WeatherAPIKey)
	params.Add("q", city)
	params.Add("days", fmt.Sprintf("%d", days))
	params.Add("lang", "ru")

	resp, err := http.Get(fmt.Sprintf("%s?%s", baseURL, params.Encode()))
	if err != nil {
		stats.IncrementErrors()
		return nil, err
	}
	defer resp.Body.Close()

	var weatherData WeatherResponse
	err = json.NewDecoder(resp.Body).Decode(&weatherData)
	if err != nil {
		stats.IncrementErrors()
		return nil, err
	}

	// Записываем время ответа API
	duration := time.Since(start)
	stats.RecordAPIResponse("weatherAPI", duration)

	return &weatherData, nil
}

// Обработчик /start
func handleStart(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	start := time.Now()
	defer func() {
		stats.Update(time.Since(start))
	}()

	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Привет! Я бот погоды. Напиши название города или выбери действие:")
	msg.ReplyMarkup = getMainKeyboard()
	bot.Send(msg)
}

// Обработчик сообщений
func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	start := time.Now()
	defer func() {
		stats.Update(time.Since(start))
	}()

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

	case "Статистика скорости":
		msg := tgbotapi.NewMessage(chatID, stats.String())
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)
		return

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

// функция для проверки наличия элемента в слайсе
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
