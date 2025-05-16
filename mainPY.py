import requests
from telegram import Update, ReplyKeyboardMarkup, KeyboardButton
from telegram.ext import Application, CommandHandler, MessageHandler, filters, CallbackContext

# Конфигурация
WEATHER_API_KEY = "38c3540b855e40cf8c2150413251505"  # Замените на свой ключ от WeatherAPI
BOT_TOKEN = "7996076983:AAHjvQBLIp-UyoLy1fT7pYu9HwJITTnD2C8"  # Получите у @BotFather

# Клавиатура для удобства
def get_main_keyboard():
    return ReplyKeyboardMarkup(
        [["Погода сейчас", "Прогноз на 3 дня"]],
        resize_keyboard=True
    )


# Клавиатура с последними городами
def get_cities_keyboard(cities):
    buttons = [[KeyboardButton(city)] for city in cities]
    buttons.append([KeyboardButton("Назад")])
    return ReplyKeyboardMarkup(buttons, resize_keyboard=True)


# Функция для запроса погоды
async def get_weather(city: str, days: int = 1) -> dict:
    url = "http://api.weatherapi.com/v1/forecast.json"
    params = {
        "key": WEATHER_API_KEY,
        "q": city,
        "days": days,
        "lang": "ru"  # Для ответа на русском
    }
    response = requests.get(url, params=params)
    return response.json()


# Обработчик команды /start
async def start(update: Update, context: CallbackContext) -> None:
    await update.message.reply_text(
        "Привет! Я бот погоды. Напиши название города или выбери действие:",
        reply_markup=get_main_keyboard()
    )


# Обработчик текстовых сообщений (город или кнопки)
async def handle_message(update: Update, context: CallbackContext) -> None:
    text = update.message.text
    user_data = context.user_data

    # Если у пользователя нет списка городов, создаем его
    if "recent_cities" not in user_data:
        user_data["recent_cities"] = []

    # Обработка кнопок
    if text == "Погода сейчас":
        if user_data["recent_cities"]:
            await update.message.reply_text(
                "Выбери город или напиши новый:",
                reply_markup=get_cities_keyboard(user_data["recent_cities"])
            )
            user_data["action"] = "current"
        else:
            await update.message.reply_text("Напиши название города:")
            user_data["action"] = "current"

    elif text == "Прогноз на 3 дня":
        if user_data["recent_cities"]:
            await update.message.reply_text(
                "Выбери город или напиши новый:",
                reply_markup=get_cities_keyboard(user_data["recent_cities"])
            )
            user_data["action"] = "forecast"
        else:
            await update.message.reply_text("Напиши название города:")
            user_data["action"] = "forecast"

    elif text == "Назад":
        await update.message.reply_text(
            "Выбери действие:",
            reply_markup=get_main_keyboard()
        )

    # Если пользователь выбрал город из списка или ввел новый
    else:
        city = text
        action = user_data.get("action")

        # Обновляем список последних городов (максимум 3)
        if city not in user_data["recent_cities"]:
            user_data["recent_cities"].insert(0, city)
            user_data["recent_cities"] = user_data["recent_cities"][:3]

        if action == "current":
            weather_data = await get_weather(city)
            current = weather_data["current"]
            location = weather_data["location"]
            message = (
                f"🌤 Погода в {location['name']} ({location['country']}):\n"
                f"🌡 Температура: {current['temp_c']}°C (ощущается как {current['feelslike_c']}°C)\n"
                f"☁ Состояние: {current['condition']['text']}\n"
                f"💨 Ветер: {current['wind_kph']} км/ч\n"
                f"💧 Влажность: {current['humidity']}%"
            )
            await update.message.reply_text(message, reply_markup=get_main_keyboard())

        elif action == "forecast":
            weather_data = await get_weather(city, days=3)
            location = weather_data["location"]
            forecast_days = weather_data["forecast"]["forecastday"]
            message = f"📆 Прогноз на 3 дня для {location['name']}:\n\n"

            for day in forecast_days:
                date = day["date"]
                max_temp = day["day"]["maxtemp_c"]
                min_temp = day["day"]["mintemp_c"]
                condition = day["day"]["condition"]["text"]
                message += (
                    f"📅 {date}:\n"
                    f"⬆ Макс: {max_temp}°C, ⬇ Мин: {min_temp}°C\n"
                    f"☁ {condition}\n\n"
                )
            await update.message.reply_text(message, reply_markup=get_main_keyboard())

        else:
            await update.message.reply_text("Выбери действие из меню 👇", reply_markup=get_main_keyboard())


# Запуск бота
def main() -> None:
    application = Application.builder().token(BOT_TOKEN).build()

    application.add_handler(CommandHandler("start", start))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

    application.run_polling()


if __name__ == "__main__":
    main()
