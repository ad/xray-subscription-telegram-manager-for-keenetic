# Xray Telegram Manager

Менеджер подключений xray с управлением через Telegram Bot для роутеров Keenetic. Позволяет легко переключаться между xray серверами через Telegram интерфейс, тестировать пинг серверов и управлять подключениями удаленно.

> 📋 **Быстрая установка**: см. [INSTALL.md](INSTALL.md) для простых инструкций

## Возможности

- 🤖 **Telegram Bot интерфейс** - управление через команды и кнопки
- 🔄 **Автоматическое переключение серверов** - выбор из списка доступных серверов
- 📊 **Тестирование пинга** - проверка скорости всех серверов с улучшенной сортировкой
- 🔗 **Поддержка подписок** - загрузка серверов из base64 ссылок
- 🔄 **Управление VPN** - включение/выключение VPN прямо из бота
- ⚡ **Автозапуск** - работает как системный сервис
- �️* **Безопасность** - авторизация только для указанного админа
- 📱 **Интерактивные кнопки** - удобное управление через inline клавиатуру
- ✨ **Умное редактирование сообщений** - чистый чат без дублирования сообщений
- � **Оптиемизация имен серверов** - автоматическое удаление повторяющихся суффиксов
- � **АКорректная обработка эмодзи** - эмодзи в кнопках не обрезаются
- � **Алфавинтная сортировка** - серверы отсортированы для удобного поиска
- 🔄 **Автообновление** - обновление бота через команду `/update`

## Быстрая установка на Keenetic

> ✅ **Протестировано на MediaTek MT7621** - работает без проблем!

### Метод 1: Быстрая установка одной командой (рекомендуется)

```bash
# Автоматическая установка одной командой (работает на всех роутерах Keenetic)
curl -fsSL https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh | sh

# Или с wget
wget -qO- https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh | sh

# Альтернативный способ (если pipe не работает)
wget https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh
chmod +x quick-install.sh && ./quick-install.shttps://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh
chmod +x quick-install.sh && ./quick-install.sh
```

После установки отредактируйте конфигурацию:
```bash
nano /opt/etc/xray-manager/config.json
```

### Метод 2: Установка из архива

### Предварительные требования

1. **Entware установлен** на роутере Keenetic
2. **SSH доступ** к роутеру
3. **Xray уже установлен** и настроен (через пакет `xray-core`)

### Выбор архитектуры

Большинство роутеров Keenetic используют **mips-softfloat**. Если автоматическая установка не работает, попробуйте другие варианты:

- **mips-softfloat** - большинство моделей Keenetic (KN-1010, KN-1410, KN-1711, etc.)
- **mips-hardfloat** - некоторые более новые модели
- **mipsle-softfloat** - редкие модели с little-endian
- **mipsle-hardfloat** - очень редкие модели

Чтобы узнать архитектуру вашего роутера:
```bash
ssh root@192.168.1.1 "uname -m && cat /proc/cpuinfo | grep cpu"
```

### Автоматическая установка

1. **Скачайте готовый релиз** с GitHub:
   ```bash
   # Архив с бинарным файлом и скриптами (рекомендуется)
   wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest/download/xray-telegram-manager-mips-softfloat.tar.gz
   
   # Альтернативные архитектуры:
   # wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest/download/xray-telegram-manager-mips-hardfloat.tar.gz
   # wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest/download/xray-telegram-manager-mipsle-softfloat.tar.gz
   # wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest/download/xray-telegram-manager-mipsle-hardfloat.tar.gz
   ```

2. **Скопируйте на роутер и установите**:
   ```bash
   # Копируем архив на роутер
   scp -O xray-telegram-manager-mips-softfloat.tar.gz root@192.168.1.1:/tmp/
   
   # Подключаемся к роутеру
   ssh root@192.168.1.1
   
   # Распаковываем архив
   cd /tmp
   tar -xzf xray-telegram-manager-mips-softfloat.tar.gz
   
   # Устанавливаем с помощью включенного скрипта (если есть)
   if [ -f scripts/install.sh ]; then
     ./scripts/install.sh
   else
     # Ручная установка
     mkdir -p /opt/etc/xray-manager/{logs,scripts}
     mkdir -p /opt/bin
     cp xray-telegram-manager /opt/bin/
     chmod +x /opt/bin/xray-telegram-manager
   fi
   ```

3. **Создайте init скрипт** (если не создался автоматически):
   ```bash
   cat > /opt/etc/init.d/S99xray-telegram-manager << 'EOF'
#!/bin/sh

ENABLED=yes
PROCS=xray-telegram-manager
ARGS="-config /opt/etc/xray-manager/config.json"
PREARGS=""
DESC=$PROCS
PATH=/opt/sbin:/opt/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

. /opt/etc/init.d/rc.func
EOF
   
   chmod +x /opt/etc/init.d/S99xray-telegram-manager
   ```

4. **Настройте конфигурацию**:
   ```bash
   nano /opt/etc/xray-manager/config.json
   ```
   
   Заполните обязательные поля:
   - `admin_id` - ваш Telegram ID (получите у @userinfobot)
   - `bot_token` - токен бота от @BotFather
   - `subscription_url` - ссылка на вашу подписку VLESS

5. **Запустите сервис**:
   ```bash
   /opt/etc/init.d/S99xray-telegram-manager start
   ```

### Метод 3: Ручная установка только бинарного файла

```bash
# Узнайте последнюю версию на https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases
# Замените v1.0.0 на актуальную версию
wget https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/download/v1.0.0/xray-telegram-manager-v1.0.0-mips-softfloat

# Скопируйте на роутер
scp -O xray-telegram-manager-v1.0.0-mips-softfloat root@192.168.1.1:/opt/bin/xray-telegram-manager
ssh root@192.168.1.1 "chmod +x /opt/bin/xray-telegram-manager"

# Создайте конфигурацию вручную
ssh root@192.168.1.1 "mkdir -p /opt/etc/xray-manager/logs"
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
- `/list` - список всех доступных серверов (отсортированы по алфавиту)
- `/status` - текущий активный сервер и статус
- `/ping` - тестирование пинга всех серверов с улучшенным отображением результатов
- `/vpn` - управление VPN (включение/выключение Xray сервиса)
- `/update` - обновить бот до последней версии (только для администратора)

### Новые возможности интерфейса

- **Умное редактирование сообщений** - бот редактирует существующие сообщения вместо отправки новых
- **Оптимизация имен серверов** - автоматическое удаление повторяющихся суффиксов для лучшей читаемости
- **Улучшенная обработка эмодзи** - корректное отображение эмодзи в кнопках без обрезания
- **Сортировка серверов** - алфавитная сортировка в списках, сортировка по скорости в результатах пинга
- **Навигация "Назад"** - удобные кнопки возврата к предыдущим экранам

### Команда обновления

Команда `/update` позволяет администратору обновить бот до последней версии прямо из Telegram:

- Доступна только администратору (указанному в `admin_id`)
- Показывает прогресс обновления в реальном времени
- Автоматически скачивает и устанавливает последнюю версию
- В случае ошибки предоставляет детальную информацию для диагностики
- Использует тот же скрипт установки, что и при первоначальной установке

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
    "ping_timeout": 5,
    "ui": {
        "max_button_text_length": 50,
        "servers_per_page": 32,
        "max_quick_select_servers": 10,
        "message_timeout_minutes": 60,
        "enable_name_optimization": true,
        "name_optimization_threshold": 0.7
    },
    "update": {
        "script_url": "https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh",
        "timeout_minutes": 10,
        "backup_config": false
    }
}
```

### Параметры конфигурации

#### Основные параметры
- `admin_id` - **обязательно** - ваш Telegram ID
- `bot_token` - **обязательно** - токен Telegram бота
- `subscription_url` - **обязательно** - ссылка на base64 подписку VLESS
- `config_path` - путь к конфигу xray (по умолчанию: `/opt/etc/xray/configs/04_outbounds.json`)
- `log_level` - уровень логирования: `debug`, `info`, `warn`, `error`
- `xray_restart_command` - команда перезапуска xray
- `cache_duration` - время кэширования подписки в секундах
- `health_check_interval` - интервал проверки здоровья сервиса
- `ping_timeout` - таймаут для тестирования пинга

#### Настройки интерфейса (ui)
- `max_button_text_length` - максимальная длина текста кнопки (по умолчанию: 50)
- `servers_per_page` - количество серверов на странице (по умолчанию: 32)
- `max_quick_select_servers` - максимальное количество серверов в быстром выборе после пинга (по умолчанию: 10)
- `message_timeout_minutes` - время жизни активных сообщений в минутах (по умолчанию: 60)
- `enable_name_optimization` - включить оптимизацию имен серверов (по умолчанию: true)
- `name_optimization_threshold` - порог для оптимизации имен (0.7 = 70% серверов должны иметь общий суффикс)

#### Настройки обновления (update)
- `script_url` - URL скрипта для обновления (по умолчанию: GitHub репозиторий)
- `timeout_minutes` - таймаут обновления в минутах (по умолчанию: 10)
- `backup_config` - создавать резервную копию конфигурации перед обновлением (по умолчанию: false)

> 📖 **Подробная документация по конфигурации**: см. [CONFIG.md](CONFIG.md) для детального описания всех параметров

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

5. **Бинарный файл не запускается**:
   - Проверьте архитектуру: `uname -m && cat /proc/cpuinfo | grep cpu`
   - Попробуйте другую архитектуру (mips-hardfloat, mipsle-softfloat, mipsle-hardfloat)
   - Убедитесь что файл исполняемый: `chmod +x /opt/bin/xray-telegram-manager`
   - Проверьте зависимости: `ldd /opt/bin/xray-telegram-manager`

6. **Ошибка "No such file or directory" при запуске**:
   - Часто означает неправильную архитектуру
   - Попробуйте: `file /opt/bin/xray-telegram-manager` для проверки типа файла

7. **Проблемы с обновлением через `/update`**:
   - Убедитесь что вы администратор (ваш ID указан в `admin_id`)
   - Проверьте доступность интернета на роутере
   - Проверьте URL скрипта обновления в конфигурации
   - Посмотрите логи для детальной информации об ошибке

8. **Проблемы с отображением интерфейса**:
   - Если кнопки обрезаются, уменьшите `max_button_text_length` в конфигурации
   - Если слишком много серверов на странице, уменьшите `servers_per_page`
   - Для отключения оптимизации имен установите `enable_name_optimization: false`

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