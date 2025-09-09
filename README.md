# Документация Backend SpamChecker

## Оглавление

1. [Обзор проекта](#обзор-проекта)
2. [Архитектура](#архитектура)
3. [Установка и настройка](#установка-и-настройка)
4. [API Documentation](#api-documentation)
5. [Структура базы данных](#структура-базы-данных)
6. [Сервисы](#сервисы)
7. [Конфигурация](#конфигурация)
8. [Docker](#docker)
9. [Разработка](#разработка)

## Обзор проекта

SpamChecker - это система для проверки телефонных номеров на спам через различные сервисы определения номера (АОН). Система поддерживает как автоматическую проверку через Android-эмуляторы, так и интеграцию с внешними API.

### Основные возможности

- **Многопользовательская система** с ролевой моделью доступа (Admin, Supervisor, User)
- **Проверка номеров через Android-эмуляторы** (Яндекс АОН, Kaspersky Who Calls, GetContact........)
- **Интеграция с внешними API** для проверки номеров
- **Автоматическое распознавание текста** (OCR) со скриншотов
- **Планировщик задач** для автоматической проверки
- **Система уведомлений** (Telegram, Email)
- **Подробная статистика и аналитика**
- **REST API** с документацией Swagger

## Архитектура

### Технологический стек

- **Язык**: Go 1.24
- **Web Framework**: Fiber v2
- **ORM**: GORM
- **База данных**: PostgreSQL 15
- **Контейнеризация**: Docker
- **Android эмуляция**: budtmo/docker-android
- **OCR**: Tesseract
- **Аутентификация**: JWT

### Структура проекта

```
spam-checker/
├── cmd/
│   └── main.go                 # Точка входа приложения
├── internal/
│   ├── config/                 # Конфигурация
│   ├── database/              # Подключение к БД и миграции
│   ├── handlers/              # HTTP обработчики
│   ├── middleware/            # Middleware (auth, logger)
│   ├── models/                # Модели данных
│   ├── services/              # Бизнес-логика
│   ├── scheduler/             # Планировщик задач
│   ├── logger/                # Логирование
│   └── utils/                 # Утилиты
├── frontend/                  # React приложение
├── docs/                      # Swagger документация
├── screenshots/               # Хранилище скриншотов
├── docker-compose.yml         # Production конфигурация
├── Dockerfile                 # Сборка приложения
├── Makefile                   # Команды сборки
└── .env.example              # Пример конфигурации
```

## Установка и настройка

### Требования

- Go 1.24+
- PostgreSQL 15+
- Docker и Docker Compose
- Tesseract OCR
- Node.js 18+ (для frontend)

### Быстрый старт

1. **Клонирование репозитория**
```bash
git clone <repository-url>
cd spam-checker
```

2. **Настройка конфигурации**
```bash
cp .env.example .env
# Отредактируйте .env файл
```

3. **Запуск через Docker Compose**
```bash
make docker-up
```

4. **Доступ к приложению**
- Web UI: http://localhost:8080
- Swagger API: http://localhost:8080/swagger/index.html

### Ручная установка

1. **Установка зависимостей**
```bash
make install
```

2. **Запуск PostgreSQL**
```bash
docker run -d \
  --name spamchecker-db \
  -e POSTGRES_DB=spamchecker \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:15-alpine
```

3. **Сборка и запуск**
```bash
make build
make run
```

### Учетные данные по умолчанию

- **Email**: admin@spamchecker.com
- **Password**: admin123

## API Documentation

### Аутентификация

Все защищенные эндпоинты требуют JWT токен в заголовке:
```
Authorization: Bearer <token>
```

### Основные эндпоинты

#### Аутентификация
- `POST /api/v1/auth/login` - Вход в систему
- `POST /api/v1/auth/register` - Регистрация (только для админов)
- `POST /api/v1/auth/refresh` - Обновление токена

#### Управление пользователями
- `GET /api/v1/users` - Список пользователей
- `GET /api/v1/users/me` - Текущий пользователь
- `PUT /api/v1/users/me` - Обновление профиля
- `PUT /api/v1/users/me/password` - Смена пароля

#### Телефонные номера
- `GET /api/v1/phones` - Список номеров
- `POST /api/v1/phones` - Добавление номера
- `PUT /api/v1/phones/:id` - Обновление номера
- `DELETE /api/v1/phones/:id` - Удаление номера
- `POST /api/v1/phones/import` - Импорт из CSV
- `GET /api/v1/phones/export` - Экспорт в CSV

#### Проверка номеров
- `POST /api/v1/checks/phone/:id` - Проверить номер
- `POST /api/v1/checks/all` - Проверить все активные номера
- `POST /api/v1/checks/realtime` - Проверка без сохранения
- `GET /api/v1/checks/results` - История проверок
- `GET /api/v1/checks/screenshot/:id` - Получить скриншот

#### ADB Gateway
- `GET /api/v1/adb/gateways` - Список шлюзов
- `POST /api/v1/adb/gateways` - Создать шлюз
- `POST /api/v1/adb/gateways/docker` - Создать Docker-шлюз
- `POST /api/v1/adb/gateways/:id/install-apk` - Установить APK

#### API сервисы
- `GET /api/v1/api-services` - Список API сервисов
- `POST /api/v1/api-services` - Создать API сервис
- `POST /api/v1/api-services/:id/test` - Тестировать API

#### Настройки
- `GET /api/v1/settings` - Все настройки
- `PUT /api/v1/settings/:key` - Обновить настройку
- `GET /api/v1/settings/keywords` - Спам-ключевые слова
- `GET /api/v1/settings/schedules` - Расписания проверок

#### Статистика
- `GET /api/v1/statistics/overview` - Общая статистика
- `GET /api/v1/statistics/dashboard` - Статистика для дашборда
- `GET /api/v1/statistics/timeseries` - Временные ряды
- `GET /api/v1/statistics/services` - Статистика по сервисам

## Структура базы данных

### Основные таблицы

#### users
```sql
- id (PK)
- username (unique)
- email (unique)
- password (hashed)
- role (admin/supervisor/user)
- is_active
- created_at
- updated_at
- deleted_at
```

#### phone_numbers
```sql
- id (PK)
- number (unique)
- description
- is_active
- created_by (FK -> users)
- created_at
- updated_at
- deleted_at
```

#### spam_services
```sql
- id (PK)
- name (unique)
- code (unique)
- is_active
- is_custom
- created_at
- updated_at
```

#### check_results
```sql
- id (PK)
- phone_number_id (FK)
- service_id (FK)
- is_spam
- found_keywords (text[])
- screenshot
- raw_text
- raw_response
- checked_at
- created_at
```

#### adb_gateways
```sql
- id (PK)
- name (unique)
- host
- port
- device_id
- service_code
- is_active
- status
- is_docker
- container_id
- vnc_port
- last_ping
- created_at
- updated_at
```

## Сервисы

### CheckService
Основной сервис для проверки номеров. Поддерживает:
- Проверку через ADB (Android эмуляторы)
- Проверку через API
- Комбинированную проверку
- Управление очередями и конкурентностью

### ADBService
Управление Android эмуляторами:
- Создание и управление Docker контейнерами
- Выполнение ADB команд
- Симуляция входящих звонков
- Создание скриншотов
- Установка APK файлов

### APICheckService
Интеграция с внешними API:
- Поддержка различных форматов запросов
- JSONPath для извлечения данных
- Анализ ответов на наличие спам-признаков

### NotificationService
Система уведомлений:
- Telegram боты
- Email рассылка
- Шаблоны сообщений

### SchedulerService
Планировщик задач:
- Cron-выражения для расписаний
- Автоматическая проверка номеров
- Отправка отчетов

## Конфигурация

### Переменные окружения

```env
# Приложение
APP_NAME=SpamChecker
APP_PORT=8080
APP_ENV=development
LOG_LEVEL=info

# База данных
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=spamchecker

# JWT
JWT_SECRET=your-secret-key
JWT_EXPIRATION_HOURS=24

# OCR
TESSERACT_PATH=/usr/bin/tesseract
OCR_LANGUAGE=rus+eng

# Docker
DOCKER_HOST=192.168.1.2
DOCKER_PORT=2375

# Уведомления
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

### Системные настройки

Настройки хранятся в БД и управляются через API:

- `check_interval_minutes` - Интервал автоматической проверки
- `max_concurrent_checks` - Максимум параллельных проверок
- `check_mode` - Режим проверки (adb_only/api_only/both)
- `screenshot_quality` - Качество скриншотов
- `ocr_confidence_threshold` - Порог уверенности OCR

## Docker

### Production сборка

```bash
# Сборка образа
make docker-build

# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f app
```

### Docker Compose сервисы

- **postgres** - База данных PostgreSQL
- **app** - Основное приложение

### Порты

- 8080 - Web приложение
- 5433 - PostgreSQL
- 6080-6082 - VNC для эмуляторов
- 5554-5559 - ADB порты

## Разработка

### Команды Make

```bash
make install      # Установка зависимостей
make build        # Сборка приложения
make dev          # Запуск в режиме разработки
make test         # Запуск тестов
make swagger      # Генерация Swagger документации
make fmt          # Форматирование кода
make lint         # Проверка кода
```

### Добавление нового сервиса проверки

1. Создайте запись в таблице `spam_services`
2. Реализуйте логику в `CheckService`
3. Добавьте конфигурацию шлюза или API
4. Настройте ключевые слова для определения спама

### Логирование

Используется структурированное логирование через logrus:

```go
log := logger.WithFields(logrus.Fields{
    "service": "CheckService",
    "phone": phone.Number,
})
log.Info("Starting check")
```

### Обработка ошибок

Все ошибки логируются и возвращаются в структурированном виде:

```json
{
  "error": "Описание ошибки",
  "code": 400,
  "request_id": "uuid"
}
```

## Производительность и масштабирование

### Оптимизации

- Пулы подключений к БД
- Очереди для управления нагрузкой на эмуляторы
- Кэширование результатов проверок
- Параллельная обработка с ограничениями

### Рекомендации

- Используйте отдельные серверы для эмуляторов
- Настройте репликацию БД для высоких нагрузок
- Мониторинг через Prometheus/Grafana
- Регулярное резервное копирование

## Безопасность

- JWT токены с настраиваемым временем жизни
- Ролевая модель доступа
- Хеширование паролей через bcrypt
- Валидация всех входных данных
- CORS политики
- Rate limiting (планируется)