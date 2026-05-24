# kubectl-k8i

> 🇬🇧 [English documentation](README.md)

Плагин kubectl для отображения подробной информации о ресурсах узлов Kubernetes с цветовой индикацией загрузки.

## Возможности

- **Табличное представление** ресурсов узлов: поды, CPU, память, процент загрузки и метаданные
- **Цветовая индикация загрузки**: зелёный (≤60%), жёлтый (61–80%), красный (>80%)
- **Метаданные узлов**: EC2 instance ID, тип инстанса, capacity type, архитектура, зона, nodepool, nodeclaim, тип автоскейлера, возраст, taints
- **Определение автоскейлера**: Karpenter, Cluster Autoscaler (CAS) и Spot.io
- **Фильтрация и сортировка** по любому атрибуту или столбцу
- **Группировка по taints** для идентификации логических групп узлов
- **Анализ воркloadов**: показывает все воркloады (Deployment, StatefulSet, DaemonSet, Pod) на выбранных узлах с агрегированными данными CPU/памяти
- **Множественные форматы вывода**: table (по умолчанию), JSON, YAML
- **Адаптивная таблица**: столбцы подстраиваются под ширину терминала
- **Без внешних зависимостей**: автономный бинарный файл, не требует jq/awk/sed/grep
- **Кроссплатформенность**: Linux, macOS и Windows (amd64 и arm64)

## Установка

### Через krew

```bash
kubectl krew install k8i
```

### Ручная загрузка

Скачайте бинарный файл для вашей платформы со [страницы релизов](https://github.com/ViktorUJ/kubectl-k8i/releases):

| Платформа      | Файл                              |
|----------------|-----------------------------------|
| Linux amd64    | `kubectl-k8i_linux_amd64.tar.gz`  |
| Linux arm64    | `kubectl-k8i_linux_arm64.tar.gz`  |
| macOS amd64    | `kubectl-k8i_darwin_amd64.tar.gz` |
| macOS arm64    | `kubectl-k8i_darwin_arm64.tar.gz` |
| Windows amd64  | `kubectl-k8i_windows_amd64.zip`   |
| Windows arm64  | `kubectl-k8i_windows_arm64.zip`   |

Распакуйте и поместите в `$PATH`:

```bash
# Linux / macOS
tar xzf kubectl-k8i_<platform>.tar.gz
chmod +x kubectl-k8i
mv kubectl-k8i /usr/local/bin/
```

### Локальная сборка и установка

```bash
git clone https://github.com/ViktorUJ/kubectl-k8i.git
cd kubectl-k8i
make install
```

Проверка установки:

```bash
kubectl k8i --help
kubectl k8i --version
```

## Автодополнение (Shell Completion)

kubectl-k8i поддерживает автодополнение по TAB для всех флагов и параметров через стандартный механизм completion для kubectl-плагинов (kubectl 1.26+).

### Настройка

После установки плагина сгенерируйте и установите скрипт автодополнения:

```bash
# Сгенерировать и установить скрипт
kubectl k8i completion > kubectl_complete-k8i
chmod +x kubectl_complete-k8i
sudo mv kubectl_complete-k8i /usr/local/bin/
```

### Проверка

Откройте новую сессию терминала и проверьте:

```bash
kubectl k8i --<TAB><TAB>
```

Вы увидите все доступные флаги (`--context`, `--labels`, `--taints`, `--filter`, `--sort` и т.д.).

### Как это работает

При нажатии TAB после `kubectl k8i` kubectl ищет исполняемый файл `kubectl_complete-k8i` в `$PATH`. Этот скрипт вызывает `kubectl-k8i __complete` с текущими аргументами, и Cobra возвращает список вариантов автодополнения.

## Использование

```bash
kubectl k8i [флаги]
kubectl k8i analyze [флаги]
```

### Флаги

| Флаг                    | Описание                                                       | По умолчанию |
|-------------------------|----------------------------------------------------------------|--------------|
| `--context CONTEXT`     | Kubernetes-контекст                                            |              |
| `--labels SELECTOR`     | Label selector для фильтрации узлов на уровне API              |              |
| `--taints KEY[=VALUE]`  | Фильтрация узлов по taint (ключ или ключ=значение)             |              |
| `--filter ATTR=VALUE`   | Фильтрация вывода по атрибуту узла                             |              |
| `--sort COLUMN=DIR`     | Сортировка по столбцу и направлению                            | `pool=asc`   |
| `--deployment NS/NAME`  | Показать только узлы с подами данного deployment               |              |
| `--statefulset NS/NAME` | Показать только узлы с подами данного statefulset              |              |
| `--daemonset NS/NAME`   | Показать только узлы с подами данного daemonset                |              |
| `--namespace NAME`      | Показать только узлы с подами из данного namespace             |              |
| `--autoscaler VALUE`    | Показать только узлы данного автоскейлера (`karpenter`, `cluster-autoscaler`, `spotio`, `x`) | |
| `--fargate`             | Включить Fargate-узлы в вывод                                  | `false`      |
| `--color true\|false`   | Принудительно включить или выключить ANSI-цвета                | `auto`       |
| `--debug`               | Включить отладочный вывод в stderr                             | `false`      |
| `--group-by taint`      | Группировать узлы по общим наборам taints                      |              |
| `--output, -o FORMAT`   | Формат вывода: `table`, `json`, `yaml`                         | `table`      |
| `--no-headers`          | Скрыть заголовок, разделитель, метку времени и аннотации       | `false`      |
| `--version`             | Показать версию плагина                                        |              |
| `--help`                | Показать справку                                               |              |

### Атрибуты фильтрации

`ec2_type`, `instance_type`, `arch`, `zone`, `pool`, `nodeclaim`, `taint`, `autoscaler`

### Столбцы сортировки

`name`, `pods`, `cpu_req`, `cpu_lim`, `cpu_use`, `cpu_cap`, `cpu_load`, `mem_req`, `mem_lim`, `mem_use`, `mem_cap`, `mem_load`, `ec2_type`, `instance_type`, `arch`, `zone`, `pool`, `age`, `taint`, `autoscaler`

## Примеры

### Вывод таблицы по умолчанию

```bash
kubectl k8i
```

### Фильтрация по типу инстанса

```bash
kubectl k8i --filter instance_type=m5.xlarge
```

### Сортировка по загрузке CPU (по убыванию)

```bash
kubectl k8i --sort cpu_load=desc
```

### Показать только узлы Karpenter

```bash
kubectl k8i --filter autoscaler=karpenter
```

### Группировка по taints

```bash
kubectl k8i --group-by taint
```

### Использование конкретного контекста

```bash
kubectl k8i --context production
```

### Фильтрация по label selector на уровне API

```bash
kubectl k8i --labels "topology.kubernetes.io/zone=us-east-1a"
```

### Фильтрация по taint

```bash
# Показать только узлы с taint-ключом "dedicated"
kubectl k8i --taints dedicated

# Показать только узлы с taint ключ=значение "dedicated=gpu"
kubectl k8i --taints 'dedicated=gpu'
```

### Включить Fargate-узлы

```bash
kubectl k8i --fargate
```

### Отключить цвета (для пайпов)

```bash
kubectl k8i --color false
```

### Скрыть заголовки (для скриптов)

```bash
kubectl k8i --no-headers
```

### Показать узлы конкретного deployment

```bash
# Только узлы, на которых запущены поды deployment "api-server" в namespace "production"
kubectl k8i --deployment production/api-server
```

Автодополнение по TAB работает для этого флага — после `--deployment` TAB предложит список namespace, после `namespace/` TAB — список deployment в этом namespace.

### Показать узлы конкретного statefulset

```bash
# Только узлы, на которых запущены поды statefulset "postgres" в namespace "production"
kubectl k8i --statefulset production/postgres
```

Автодополнение работает аналогично `--deployment`.

### Показать узлы конкретного daemonset

```bash
# Только узлы, на которых запущены поды daemonset "fluentd" в namespace "logging"
kubectl k8i --daemonset logging/fluentd
```

Автодополнение работает аналогично `--deployment`.

### Показать узлы с подами из конкретного namespace

```bash
# Только узлы, на которых есть хотя бы один запущенный под из namespace "monitoring"
kubectl k8i --namespace monitoring
```

Автодополнение возвращает список всех доступных namespace.

### Фильтрация по типу автоскейлера

```bash
# Только узлы под управлением Karpenter
kubectl k8i --autoscaler karpenter

# Только узлы Cluster Autoscaler (EKS nodegroup)
kubectl k8i --autoscaler cluster-autoscaler

# Только узлы Spot.io
kubectl k8i --autoscaler spotio

# Только узлы без распознанного автоскейлера
kubectl k8i --autoscaler x
```

Автодополнение возвращает все допустимые значения: `karpenter`, `cluster-autoscaler`, `spotio`, `x`.

### Комбинирование фильтров по воркload с другими флагами

```bash
# Узлы deployment, отсортированные по загрузке памяти
kubectl k8i --deployment production/api-server --sort mem_load=desc

# Узлы statefulset, вывод в JSON
kubectl k8i --statefulset production/postgres -o json

# Узлы daemonset, сгруппированные по taints
kubectl k8i --daemonset logging/fluentd --group-by taint

# Узлы namespace, сгруппированные по taints
kubectl k8i --namespace monitoring --group-by taint
```

## Подкоманда analyze

`kubectl k8i analyze` показывает все воркloады, запущенные на выбранном наборе узлов. Для каждого воркloада отображается namespace, тип, имя, количество подов и агрегированные CPU/память (requests, limits, usage).

### Флаги analyze

| Флаг                          | Описание                                                        | По умолчанию |
|-------------------------------|-----------------------------------------------------------------|--------------|
| `--node NAME`                 | Анализировать воркloады на конкретном узле                      |              |
| `--labels SELECTOR`           | Анализировать воркloады на узлах по label selector              |              |
| `--taints KEY[=VALUE]`        | Анализировать воркloады на узлах с данным taint                 |              |
| `--autoscaler VALUE`          | Анализировать воркloады на узлах данного автоскейлера (`karpenter`, `cluster-autoscaler`, `spotio`, `x`) | |
| `--exclude-namespace NAME`    | Исключить namespace из вывода (флаг можно указывать несколько раз) |           |
| `--output, -o FORMAT`         | Формат вывода: `table`, `json`, `yaml`                          | `table`      |
| `--color true\|false`         | Принудительно включить или выключить ANSI-цвета                 | `auto`       |
| `--context CONTEXT`           | Kubernetes-контекст                                             |              |
| `--debug`                     | Включить отладочный вывод в stderr                              | `false`      |

Необходимо указать ровно один из флагов: `--node`, `--labels`, `--taints` или `--autoscaler`.

Результаты сортируются по namespace → тип → имя.

### Примеры analyze

```bash
# Анализ воркloadов на конкретном узле
kubectl k8i analyze --node ip-10-0-1-100

# Анализ воркloadов на узлах по label selector
kubectl k8i analyze --labels 'worker-type=spot'

# Анализ воркloadов на узлах с конкретным taint
kubectl k8i analyze --taints 'dedicated=gpu'

# Анализ воркloadов на всех узлах Karpenter
kubectl k8i analyze --autoscaler karpenter

# Анализ воркloadов на узлах EKS nodegroup (CAS)
kubectl k8i analyze --autoscaler cluster-autoscaler

# Анализ воркloadов на узлах Spot.io, исключить системные namespace
kubectl k8i analyze --autoscaler spotio --exclude-namespace kube-system

# Исключить системные namespace для уменьшения шума
kubectl k8i analyze --labels 'worker-type=spot' \
  --exclude-namespace kube-system \
  --exclude-namespace monitoring

# Вывод в JSON
kubectl k8i analyze --node ip-10-0-1-100 -o json

# Вывод в YAML
kubectl k8i analyze --autoscaler karpenter -o yaml
```

### Формат таблицы analyze

```
NAMESPACE            KIND         NAME                                PODS  CPU req/lim/use    MEM req/lim/use GB
                                                                            (cores)
========================================================================================
production           Deployment   api-server                             3  0.75/1.50/0.42     1.00/2.00/0.65
production           StatefulSet  postgres                               2  0.50/1.00/0.31     2.00/4.00/1.80
kube-system          DaemonSet    aws-node                               1  0.02/0.00/0.01     0.03/0.00/0.02
```

### JSON-вывод analyze

```bash
kubectl k8i analyze --node ip-10-0-1-100 -o json
```

```json
[
  {
    "namespace": "production",
    "kind": "Deployment",
    "name": "api-server",
    "pod_count": 3,
    "cpu_request_cores": 0.75,
    "cpu_limit_cores": 1.5,
    "cpu_usage_cores": 0.42,
    "mem_request_gb": 1.0,
    "mem_limit_gb": 2.0,
    "mem_usage_gb": 0.65
  }
]
```

## Форматы вывода

### Таблица (по умолчанию)

```
2024-01-15 10:30:00 UTC
Filter: instance_type=m5.xlarge | Sort: pool=asc
NODE                PODS   CPU(req/lim/use/cap)  CPU%  MEM(req/lim/use/cap)  MEM%  EC2               TYPE        CAP  ARCH   AZ  POOL            NODECLAIM            AS        AGE   TAINTS
----                ----   --------------------  ----  --------------------  ----  ---               ----        ---  ----   --  ----            ---------            --        ---   ------
ip-10-0-1-100       12/58  3.2/6.0/2.8/4.0       70    8.5/12.0/7.2/16.0     45    i-0abc123def456  m5.xlarge   od   amd64  1a  my-pool         my-nodeclaim         karpenter 5d12h none
```

### JSON

```bash
kubectl k8i -o json
```

```json
[
  {
    "name": "ip-10-0-1-100",
    "pods_used": 12,
    "pods_max": 58,
    "cpu_request_cores": 3.2,
    "cpu_limit_cores": 6.0,
    "cpu_usage_cores": 2.8,
    "cpu_capacity_cores": 4.0,
    "cpu_load_percent": 70,
    "mem_request_gb": 8.5,
    "mem_limit_gb": 12.0,
    "mem_usage_gb": 7.2,
    "mem_capacity_gb": 16.0,
    "mem_load_percent": 45,
    "ec2_instance_id": "i-0abc123def456",
    "instance_type": "m5.xlarge",
    "capacity_type": "od",
    "architecture": "amd64",
    "zone": "1a",
    "nodepool": "my-pool",
    "nodeclaim": "my-nodeclaim",
    "autoscaler": "karpenter",
    "age": "5d12h",
    "taints": "none"
  }
]
```

### YAML

```bash
kubectl k8i -o yaml
```

```yaml
- name: ip-10-0-1-100
  pods_used: 12
  pods_max: 58
  cpu_request_cores: 3.2
  cpu_limit_cores: 6.0
  cpu_usage_cores: 2.8
  cpu_capacity_cores: 4.0
  cpu_load_percent: 70
  mem_request_gb: 8.5
  mem_limit_gb: 12.0
  mem_usage_gb: 7.2
  mem_capacity_gb: 16.0
  mem_load_percent: 45
  ec2_instance_id: i-0abc123def456
  instance_type: m5.xlarge
  capacity_type: od
  architecture: amd64
  zone: "1a"
  nodepool: my-pool
  nodeclaim: my-nodeclaim
  autoscaler: karpenter
  age: 5d12h
  taints: none
```

## Разработка

### Требования

- [Go 1.26+](https://go.dev/dl/)
- [golangci-lint](https://golangci-lint.run/welcome/install/)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) — проверка уязвимых зависимостей
- [gosec](https://github.com/securego/gosec) — статический анализ безопасности
- [goreleaser](https://goreleaser.com/install/) (опционально, для локальных релизных сборок)

Установка инструментов безопасности:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

### Сборка из исходников

```bash
git clone https://github.com/ViktorUJ/kubectl-k8i.git
cd kubectl-k8i
make build
```

### Доступные Make-таргеты

| Команда              | Описание                                                     |
|----------------------|--------------------------------------------------------------|
| `make build`         | Сборка бинарного файла для текущей платформы                 |
| `make install`       | Сборка и установка в `/usr/local/bin` (или `$GOPATH/bin`) без krew |
| `make test`          | Запуск юнит-тестов с race detector                           |
| `make lint`          | Запуск golangci-lint (статический анализ + security-линтеры) |
| `make vet`           | Запуск `go vet` (встроенный статический анализ)              |
| `make security`      | Запуск govulncheck + gosec (проверка уязвимостей и безопасности) |
| `make vulncheck`     | Проверка зависимостей на известные уязвимости (govulncheck)  |
| `make check`         | Все проверки: lint + vet + security                          |
| `make test-all`      | Все проверки + все тесты (unit, integration, e2e)            |
| `make build-all`     | Кросс-компиляция для всех 6 платформ в `dist/`              |
| `make release-local` | Локальная релизная сборка через GoReleaser (snapshot)         |
| `make clean`         | Удаление артефактов сборки                                   |

### Проверка безопасности

```bash
# Проверка зависимостей на известные уязвимости
make vulncheck

# Полная проверка безопасности (govulncheck + gosec)
make security

# Весь статический анализ (lint + vet + security)
make check
```

### Создание релиза

Релизы полностью автоматизированы. При push в `main` CI-пайплайн:

1. Запускает линтинг, тесты и сборку для всех 6 платформ
2. Автоматически создаёт тег с инкрементом patch-версии (например, `v0.1.0` → `v0.1.1`)
3. Новый тег запускает релизный workflow, который через GoReleaser создаёт GitHub Release с кросс-компилированными бинарниками и обновлённым krew-манифестом

Для ручного создания релиза:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Требования к окружению

- Работающий кластер Kubernetes, доступный через `kubectl`
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server) для данных об использовании CPU/памяти (опционально — без него значения использования будут нулевыми)

## Лицензия

Apache License 2.0. Подробности в файле [LICENSE](LICENSE).
