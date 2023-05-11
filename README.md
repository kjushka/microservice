### ШАБЛОН МИКРОСЕРВИСА
Включает:
1. Полноценный Makefile
2. Настроенные экшены для Github
3. Сторедж данных с кэшом
4. Rate Limiter и Circuit Breaker
5. Вспомогательные средства (логгер, клоузер и тд)
6. Миграции
7. Graylog, Prometheus, Grafana, NGINX
8. Генерация из swagger и proto

Чтобы использовать генерацию нужно на MacOS
1. brew install protobuf

Для запуска всей системы нужно:
1. go mod download
2. make build
3. make run

Для остановки: make down