# Быстрая установка

## Метод 1: Одна команда (самый простой)

Скопируйте и выполните эту команду на роутере:

```bash
curl -fsSL https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh | bash
```

Или если curl недоступен:

```bash
wget -qO- https://raw.githubusercontent.com/ad/xray-subscription-telegram-manager-for-keenetic/main/scripts/quick-install.sh | bash
```

## Метод 2: Загрузка и установка

1. **Скачайте архив:**
   - Перейдите на https://github.com/ad/xray-subscription-telegram-manager-for-keenetic/releases/latest
   - Скачайте `xray-telegram-manager-mips-softfloat.tar.gz` (для большинства роутеров)

2. **Установите:**
   ```bash
   # Скопируйте на роутер
   scp xray-telegram-manager-mips-softfloat.tar.gz root@192.168.1.1:/tmp/
   
   # Подключитесь к роутеру и установите
   ssh root@192.168.1.1
   cd /tmp && tar -xzf xray-telegram-manager-mips-softfloat.tar.gz
   ./scripts/install.sh  # Если есть скрипт установки
   ```

## После установки

1. **Настройте бота:**
   ```bash
   nano /opt/etc/xray-manager/config.json
   ```

2. **Получите нужные данные:**
   - Telegram ID: напишите @userinfobot
   - Bot Token: напишите @BotFather, создайте бота

3. **Запустите:**
   ```bash
   /opt/etc/init.d/S99xray-telegram-manager start
   ```

## Архитектуры роутеров

- **mips-softfloat** - большинство Keenetic (попробуйте сначала это)
- **mips-hardfloat** - если первый не работает
- **mipsle-softfloat** - редкие модели
- **mipsle-hardfloat** - очень редкие модели

Если не уверены в архитектуре:
```bash
uname -m
```

## Проблемы?

- **"No such file"** → неправильная архитектура, попробуйте другую
- **Бот не отвечает** → проверьте токен и admin_id в config.json
- **Нет интернета** → проверьте настройки роутера

Полная документация: [README.md](README.md)
