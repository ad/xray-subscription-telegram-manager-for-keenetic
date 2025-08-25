# Xray Telegram Manager

Менеджер подключений xray с управлением через Telegram Bot для роутеров Keenetic. Позволяет легко переключаться между xray серверами через Telegram интерфейс, тестировать пинг серверов и управлять подключениями удаленно.

## Возможности

- 🤖 **Telegram Bot интерфейс** - управление через команды и кнопки
- 🔄 **Автоматическое переключение серверов** - выбор из списка доступных серверов
- 📊 **Тестирование пинга** - проверка скорости всех серверов с сортировкой
- 🔗 **Поддержка подписок** - загрузка серверов из base64 ссылок
- ⚡ **Автозапуск** - работает как системный сервис
- 🛡️ **Безопасность** - авторизация только для указанного админа
- 📱 **Интерактивные кнопки** - удобное управление через inline клавиатуру

## Быстрая установка на Keenetic

### Предварительные требования

1. **Entware установлен** на роутере Keenetic
2. **SSH доступ** к роутеру
3. **Xray уже установлен** и настроен (через пакет `xray-core`)

### Автоматическая установка

1. **Скачайте готовый релиз** с GitHub:
   ```bash
   # На вашем компьютере
   wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest/download/xray-telegram-manager-mips-softfloat.tar.gz
   ```

2. **Скопируйте на роутер и установите**:
   ```bash
   # Копируем архив на роутер
   scp xray-telegram-manager-mips-softfloat.tar.gz root@192.168.1.1:/tmp/
   
   # Подключаемся к роутеру
   ssh root@192.168.1.1
   
   # Распаковываем и устанавливаем
   cd /tmp
   tar -xzf xray-telegram-manager-mips-softfloat.tar.gz
   ./scripts/install.sh
   ```

3. **Настройте конфигурацию**:
   ```bash
   nano /opt/etc/xray-manager/config.json
   ```
   
   Заполните обязательные поля:
   - `admin_id` - ваш Telegram ID (получите у @userinfobot)
   - `bot_token` - токен бота от @BotFather
   - `subscription_url` - ссылка на вашу подписку VLESS

4. **Запустите сервис**:
   ```bash
   /opt/etc/init.d/S99xray-telegram-manager start
   ```

### Создание Telegram бота

1. Напишите @BotFather в Telegram
2. Отправьте команду `/newbot`
3. Следуйте инструкциям для создания бота
4. Скопируйте полученный токен в конфигурацию
5. Узнайте свой Telegram ID у @userinfobot

### Проверка установки

```bash
# Проверить статус сервиса
/opt/etc/init.d/S99xray-telegram-manager status

# Посмотреть логи
tail -f /opt/etc/xray-manager/logs/app.log

# Проверить процесс
ps | grep xray-telegram-manager
```

## Команды Telegram бота

После успешной установки отправьте боту:

- `/start` - показать список серверов с кнопками выбора
- `/list` - список всех доступных серверов
- `/status` - текущий активный сервер и статус
- `/ping` - тестирование пинга всех серверов

## Ручная сборка и установка

### Сборка из исходников

```bash
# Клонируем репозиторий
git clone https://github.com/ad/xray-subscription-telegram-manager-for-keenetic.git
cd xray-subscription-telegram-manager-for-keenetic

# Собираем для MIPS
make mips

# Или используем скрипт развертывания
./scripts/deploy.sh --target 192.168.1.1
```

### Сборка для разных архитектур

```bash
# MIPS softfloat (большинство Keenetic)
make mips-softfloat

# MIPS hardfloat (некоторые модели)
make mips-hardfloat

# Локальная разработка
make build
```

## Конфигурация

Полный пример конфигурации `/opt/etc/xray-manager/config.json`:

```json
{
    "admin_id": 123456789,
    "bot_token": "1234567890:ABCdefGHIjklMNOpqrsTUVwxyz",
    "config_path": "/opt/etc/xray/configs/04_outbounds.json",
    "subscription_url": "https://example.com/subscription.txt",
    "log_level": "info",
    "xray_restart_command": "/opt/etc/init.d/S24xray restart",
    "cache_duration": 3600,
    "health_check_interval": 300,
    "ping_timeout": 5
}
```

### Параметры конфигурации

- `admin_id` - **обязательно** - ваш Telegram ID
- `bot_token` - **обязательно** - токен Telegram бота
- `subscription_url` - **обязательно** - ссылка на base64 подписку VLESS
- `config_path` - путь к конфигу xray (по умолчанию: `/opt/etc/xray/configs/04_outbounds.json`)
- `log_level` - уровень логирования: `debug`, `info`, `warn`, `error`
- `xray_restart_command` - команда перезапуска xray
- `cache_duration` - время кэширования подписки в секундах
- `health_check_interval` - интервал проверки здоровья сервиса
- `ping_timeout` - таймаут для тестирования пинга

## Управление сервисом

```bash
# Запуск
/opt/etc/init.d/S99xray-telegram-manager start

# Остановка
/opt/etc/init.d/S99xray-telegram-manager stop

# Перезапуск
/opt/etc/init.d/S99xray-telegram-manager restart

# Статус
/opt/etc/init.d/S99xray-telegram-manager status

# Включить автозапуск
/opt/etc/init.d/S99xray-telegram-manager enable

# Отключить автозапуск
/opt/etc/init.d/S99xray-telegram-manager disable
```

## Устранение неполадок

### Проверка логов

```bash
# Основные логи приложения
tail -f /opt/etc/xray-manager/logs/app.log

# Системные логи (если используется systemd)
journalctl -u xray-telegram-manager -f
```

### Частые проблемы

1. **Бот не отвечает**:
   - Проверьте токен бота в конфигурации
   - Убедитесь что сервис запущен
   - Проверьте интернет-соединение роутера

2. **Ошибка авторизации**:
   - Проверьте правильность `admin_id` в конфигурации
   - Получите свой ID у @userinfobot

3. **Не загружаются серверы**:
   - Проверьте доступность `subscription_url`
   - Убедитесь что ссылка содержит base64 данные

4. **Не переключаются серверы**:
   - Проверьте путь к конфигу xray (`config_path`)
   - Убедитесь что команда перезапуска xray работает

### Полное удаление

```bash
# Остановить и удалить сервис
/opt/etc/xray-manager/scripts/uninstall.sh

# Или вручную
/opt/etc/init.d/S99xray-telegram-manager stop
rm -rf /opt/etc/xray-manager
rm -f /opt/etc/init.d/S99xray-telegram-manager
```

## Архитектура проекта

```
├── config/          # Управление конфигурацией
├── telegram/        # Telegram bot интерфейс
├── server/          # Управление серверами и подписками
├── xray/            # Управление конфигурацией xray
├── logger/          # Система логирования
├── scripts/         # Скрипты установки и развертывания
└── main.go          # Основная точка входа
```

## Зависимости

- `github.com/go-telegram/bot` - единственная внешняя зависимость для Telegram API

## Лицензия

MIT License - см. файл [LICENSE](LICENSE)