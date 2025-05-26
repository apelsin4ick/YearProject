const TelegramBot = require('node-telegram-bot-api');
const axios = require('axios');

// Конфигурация
const WEATHER_API_KEY = "38c3540b855e40cf8c2150413251505"; 
const BOT_TOKEN = "7800321238:AAGpNKr1CaYGweApXoft2HFCrwcR_Fardw8"; 

// Создаем экземпляр бота
const bot = new TelegramBot(BOT_TOKEN, {polling: true});

// Клавиатура для удобства
function getMainKeyboard() {
    return {
        keyboard: [["Погода сейчас", "Прогноз на 3 дня"]],
        resize_keyboard: true
    };
}

// Клавиатура с последними городами
function getCitiesKeyboard(cities) {
    const buttons = cities.map(city => [{text: city}]);
    buttons.push([{text: "Назад"}]);
    return {
        keyboard: buttons,
        resize_keyboard: true
    };
}

// Функция для запроса погоды
async function getWeather(city, days = 1) {
    const url = "http://api.weatherapi.com/v1/forecast.json";
    const params = {
        key: WEATHER_API_KEY,
        q: city,
        days: days,
        lang: "ru"
    };
    
    try {
        const response = await axios.get(url, {params});
        return response.data;
    } catch (error) {
        console.error("Error fetching weather:", error);
        return null;
    }
}

// Обработчик /start
bot.onText(/\/start/, (msg) => {
    const chatId = msg.chat.id;
    bot.sendMessage(
        chatId,
        "Привет! Я бот погоды. Напиши название города или выбери действие:",
        {
            reply_markup: getMainKeyboard()
        }
    );
});

// Обработчик сообщений (город или кнопки)
bot.on('message', async (msg) => {
    const chatId = msg.chat.id;
    const text = msg.text;
    
    // Пропускаем команду /start, так как у нее есть отдельный обработчик
    if (text.startsWith('/')) return;
    
    // Инициализируем userData если его нет
    if (!bot.userData) bot.userData = {};
    if (!bot.userData[chatId]) bot.userData[chatId] = {};
    
    const userData = bot.userData[chatId];
    
    // Если у пользователя нет списка городов, создаем его
    if (!userData.recentCities) {
        userData.recentCities = [];
    }

    // Обработка кнопок
    if (text === "Погода сейчас") {
        if (userData.recentCities.length > 0) {
            bot.sendMessage(
                chatId,
                "Выбери город или напиши новый:",
                {
                    reply_markup: getCitiesKeyboard(userData.recentCities)
                }
            );
            userData.action = "current";
        } else {
            bot.sendMessage(chatId, "Напиши название города:");
            userData.action = "current";
        }
    } else if (text === "Прогноз на 3 дня") {
        if (userData.recentCities.length > 0) {
            bot.sendMessage(
                chatId,
                "Выбери город или напиши новый:",
                {
                    reply_markup: getCitiesKeyboard(userData.recentCities)
                }
            );
            userData.action = "forecast";
        } else {
            bot.sendMessage(chatId, "Напиши название города:");
            userData.action = "forecast";
        }
    } else if (text === "Назад") {
        bot.sendMessage(
            chatId,
            "Выбери действие:",
            {
                reply_markup: getMainKeyboard()
            }
        );
    } else {
        // Если пользователь выбрал город из списка или ввел новый
        const city = text;
        const action = userData.action;

        // Обновляем список последних городов (максимум 3)
        if (!userData.recentCities.includes(city)) {
            userData.recentCities.unshift(city);
            userData.recentCities = userData.recentCities.slice(0, 3);
        }

        if (action === "current") {
            const weatherData = await getWeather(city);
            if (!weatherData) {
                bot.sendMessage(chatId, "Не удалось получить данные о погоде. Попробуйте другой город.");
                return;
            }
            
            const current = weatherData.current;
            const location = weatherData.location;
            const message = (
                `🌤 Погода в ${location.name} (${location.country}):\n` +
                `🌡 Температура: ${current.temp_c}°C (ощущается как ${current.feelslike_c}°C)\n` +
                `☁ Состояние: ${current.condition.text}\n` +
                `💨 Ветер: ${current.wind_kph} км/ч\n` +
                `💧 Влажность: ${current.humidity}%`
            );
            bot.sendMessage(chatId, message, {reply_markup: getMainKeyboard()});

        } else if (action === "forecast") {
            const weatherData = await getWeather(city, 3);
            if (!weatherData) {
                bot.sendMessage(chatId, "Не удалось получить прогноз погоды. Попробуйте другой город.");
                return;
            }
            
            const location = weatherData.location;
            const forecastDays = weatherData.forecast.forecastday;
            let message = `📆 Прогноз на 3 дня для ${location.name}:\n\n`;

            for (const day of forecastDays) {
                const date = day.date;
                const maxTemp = day.day.maxtemp_c;
                const minTemp = day.day.mintemp_c;
                const condition = day.day.condition.text;
                message += (
                    `📅 ${date}:\n` +
                    `⬆ Макс: ${maxTemp}°C, ⬇ Мин: ${minTemp}°C\n` +
                    `☁ ${condition}\n\n`
                );
            }
            bot.sendMessage(chatId, message, {reply_markup: getMainKeyboard()});
        } else {
            bot.sendMessage(
                chatId,
                "Выбери действие из меню 👇",
                {
                    reply_markup: getMainKeyboard()
                }
            );
        }
//
    }
});

console.log("Бот запущен и ожидает сообщений...");
