import logging
import time
from datetime import datetime
from typing import Dict, List, Optional, Literal

import requests
from telegram import ReplyKeyboardMarkup, Update
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    filters,
    ContextTypes,
)

# Конфигурация
WEATHER_API_KEY = "38c3540b855e40cf8c2150413251505"
BOT_TOKEN = "7996076983:AAHjvQBLIp-UyoLy1fT7pYu9HwJITTnD2C8"

# Настройка логирования
logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s", level=logging.INFO
)
logger = logging.getLogger(__name__)

# Типы для аннотаций
WeatherAction = Literal["current", "forecast"]

class PerformanceStats:
    def __init__(self):
        self.total_requests = 0
        self.total_response_time = 0
        self.api_response_times = []
        self.last_10_response_times = []
        self.errors = 0

    def update(self, response_time: int):
        self.total_requests += 1
        self.total_response_time += response_time
        self.last_10_response_times.append(response_time)
        if len(self.last_10_response_times) > 10:
            self.last_10_response_times.pop(0)

    def __str__(self):
        avg_response = (self.total_response_time / self.total_requests) if self.total_requests > 0 else 0
        last_10_avg = (sum(self.last_10_response_times) / len(self.last_10_response_times)) if self.last_10_response_times else 0
        last_api = self.api_response_times[-1]["time"] if self.api_response_times else "нет данных"
        
        return (
            f"📊 Статистика:\n"
            f"• Запросов: {self.total_requests}\n"
            f"• Среднее время: {avg_response:.2f} мс\n"
            f"• Последние 10: {last_10_avg:.2f} мс\n"
            f"• Ошибок: {self.errors}\n"
            f"• Последний API: {last_api} мс"
        )

class UserData:
    def __init__(self):
        self.recent_cities: List[str] = []
        self.action: Optional[WeatherAction] = None

# Глобальные объекты
performance_stats = PerformanceStats()
user_states: Dict[int, UserData] = {}

def get_main_keyboard():
    return ReplyKeyboardMarkup(
        [["Погода сейчас", "Прогноз на 3 дня"], ["Статистика скорости"]],
        resize_keyboard=True,
    )

def get_cities_keyboard(cities: List[str]):
    buttons = [[city] for city in cities]
    buttons.append(["Назад"])
    return ReplyKeyboardMarkup(buttons, resize_keyboard=True)

async def get_weather(city: str, days: int = 1) -> Optional[Dict]:
    start_time = time.time()
    url = "http://api.weatherapi.com/v1/forecast.json"
    params = {
        "key": WEATHER_API_KEY,
        "q": city,
        "days": days,
        "lang": "ru",
    }

    try:
        response = requests.get(url, params=params)
        response_time = int((time.time() - start_time) * 1000)
        
        performance_stats.api_response_times.append({
            "endpoint": "weatherAPI",
            "time": response_time,
            "timestamp": datetime.now().isoformat(),
        })
        
        return response.json()
    except Exception as error:
        logger.error(f"Weather API error: {error}")
        performance_stats.errors += 1
        return None

async def handle_weather_response(update: Update, city: str, days: int):
    weather_data = await get_weather(city, days)
    if not weather_data:
        await update.message.reply_text("Не удалось получить данные о погоде.")
        return False

    location = weather_data["location"]
    if days == 1:
        current = weather_data["current"]
        message = (
            f"🌤 Погода в {location['name']} ({location['country']}):\n"
            f"🌡 Температура: {current['temp_c']}°C\n"
            f"☁ Состояние: {current['condition']['text']}\n"
            f"💨 Ветер: {current['wind_kph']} км/ч\n"
            f"💧 Влажность: {current['humidity']}%"
        )
    else:
        forecast_days = weather_data["forecast"]["forecastday"]
        message = f"📆 Прогноз на 3 дня для {location['name']}:\n\n"
        for day in forecast_days:
            message += (
                f"📅 {day['date']}:\n"
                f"⬆ Макс: {day['day']['maxtemp_c']}°C\n"
                f"⬇ Мин: {day['day']['mintemp_c']}°C\n"
                f"☁ {day['day']['condition']['text']}\n\n"
            )

    await update.message.reply_text(message, reply_markup=get_main_keyboard())
    return True

async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    start_time = time.time()
    chat_id = update.effective_chat.id
    
    if chat_id not in user_states:
        user_states[chat_id] = UserData()
    
    await update.message.reply_text(
        "Привет! Я бот погоды. Выбери действие:",
        reply_markup=get_main_keyboard(),
    )
    
    performance_stats.update(int((time.time() - start_time) * 1000))

async def handle_message(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    start_time = time.time()
    chat_id = update.effective_chat.id
    text = update.message.text

    if text.startswith("/"):
        return

    # Инициализация или получение состояния пользователя
    user = user_states.setdefault(chat_id, UserData())

    if text == "Погода сейчас":
        if user.recent_cities:
            await update.message.reply_text(
                "Выбери город:",
                reply_markup=get_cities_keyboard(user.recent_cities),
            )
        else:
            await update.message.reply_text("Напиши название города:")
        user.action = "current"

    elif text == "Прогноз на 3 дня":
        if user.recent_cities:
            await update.message.reply_text(
                "Выбери город:",
                reply_markup=get_cities_keyboard(user.recent_cities),
            )
        else:
            await update.message.reply_text("Напиши название города:")
        user.action = "forecast"

    elif text == "Статистика скорости":
        await update.message.reply_text(str(performance_stats))

    elif text == "Назад":
        await update.message.reply_text("Выбери действие:", reply_markup=get_main_keyboard())

    else:
        # Обработка ввода города
        city = text
        if city not in user.recent_cities:
            user.recent_cities.insert(0, city)
            user.recent_cities = user.recent_cities[:3]

        if user.action:
            success = await handle_weather_response(
                update, 
                city, 
                days=3 if user.action == "forecast" else 1
            )
            if success:
                user.action = None

    performance_stats.update(int((time.time() - start_time) * 1000))

def main() -> None:
    application = Application.builder().token(BOT_TOKEN).build()
    application.add_handler(CommandHandler("start", start))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))
    logger.info("Бот запущен")
    application.run_polling()

if __name__ == "__main__":
    main()