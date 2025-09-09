# SpamChecker Frontend Documentation

## Оглавление

1. [Обзор](#обзор)
2. [Технологический стек](#технологический-стек)
3. [Архитектура проекта](#архитектура-проекта)
4. [Установка и запуск](#установка-и-запуск)
5. [Структура проекта](#структура-проекта)
6. [Основные компоненты](#основные-компоненты)
7. [Управление состоянием](#управление-состоянием)
8. [Маршрутизация](#маршрутизация)
9. [Интернационализация](#интернационализация)
10. [API интеграция](#api-интеграция)
11. [Темы и стилизация](#темы-и-стилизация)
12. [Безопасность](#безопасность)

## Обзор

SpamChecker Frontend - это современное веб-приложение для мониторинга телефонных номеров компании на предмет спама. Приложение предоставляет удобный интерфейс для управления телефонными номерами, проверки их статуса в различных сервисах и получения аналитики.

### Основные функции

- 📱 **Управление телефонными номерами** - добавление, редактирование, импорт/экспорт
- 🔍 **Проверка на спам** - проверка номеров через ADB-шлюзы и API сервисы
- 📊 **Статистика и аналитика** - визуализация данных о проверках
- 👥 **Управление пользователями** - ролевая модель доступа
- ⚙️ **Настройки системы** - конфигурация шлюзов, API, расписаний
- 🌐 **Мультиязычность** - поддержка русского и английского языков

## Технологический стек

### Основные технологии

- **React 18.2** - основной фреймворк
- **TypeScript 4.9** - типизация и безопасность кода
- **Material-UI 5.15** - библиотека компонентов
- **MobX 6.12** - управление состоянием
- **React Router 6.21** - маршрутизация

### Дополнительные библиотеки

- **Axios** - HTTP клиент
- **Recharts** - графики и диаграммы
- **i18next** - интернационализация
- **date-fns** - работа с датами
- **Notistack** - уведомления
- **Papaparse** - парсинг CSV файлов

## Архитектура проекта

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│   Components    │────▶│     Stores      │────▶│   API Client    │
│                 │     │     (MobX)      │     │    (Axios)      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│     Pages       │     │     Utils       │     │   Backend API   │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Принципы архитектуры

1. **Компонентный подход** - переиспользуемые UI компоненты
2. **Реактивное состояние** - MobX для управления данными
3. **Типобезопасность** - строгая типизация TypeScript
4. **Разделение ответственности** - четкое разделение логики и представления

## Установка и запуск

### Требования

- Node.js 16+
- npm или yarn

### Установка зависимостей

```bash
cd frontend
npm install
```

### Разработка

```bash
npm start
# Приложение будет доступно на http://localhost:3000
```

### Сборка для продакшена

```bash
npm run build
# Сборка будет создана в папке build/
```

### Генерация иконок

```bash
./generate-icons.sh
```

## Структура проекта

```
frontend/
├── public/                 # Статические файлы
│   ├── index.html         # HTML шаблон
│   ├── favicon.ico        # Фавикон
│   ├── logo192.png        # Логотип 192x192
│   ├── logo512.png        # Логотип 512x512
│   └── manifest.json      # PWA манифест
├── src/
│   ├── components/        # Переиспользуемые компоненты
│   │   ├── Layout.tsx    # Основной макет приложения
│   │   ├── PrivateRoute.tsx  # Защищенные маршруты
│   │   └── LoadingScreen.tsx # Экран загрузки
│   ├── pages/            # Страницы приложения
│   │   ├── LoginPage.tsx      # Страница входа
│   │   ├── RegisterPage.tsx   # Страница регистрации
│   │   ├── DashboardPage.tsx  # Главная панель
│   │   ├── PhonesPage.tsx     # Управление номерами
│   │   ├── ChecksPage.tsx     # Проверки номеров
│   │   ├── StatisticsPage.tsx # Статистика
│   │   ├── UsersPage.tsx      # Управление пользователями
│   │   ├── SettingsPage.tsx   # Настройки системы
│   │   └── NotFoundPage.tsx   # 404 страница
│   ├── stores/           # MobX хранилища
│   │   ├── AuthStore.ts      # Авторизация
│   │   └── PhoneStore.ts     # Управление телефонами
│   ├── i18n/             # Интернационализация
│   │   ├── index.ts          # Конфигурация i18n
│   │   └── locales/          # Переводы
│   │       ├── en.ts         # Английский
│   │       └── ru.ts         # Русский
│   ├── App.tsx           # Главный компонент
│   ├── index.tsx         # Точка входа
│   └── index.css         # Глобальные стили
├── package.json          # Зависимости проекта
└── tsconfig.json         # Конфигурация TypeScript
```

## Основные компоненты

### Layout

Основной макет приложения с боковой панелью навигации.

```typescript
// Использование
<Layout>
  <Outlet /> {/* Контент страницы */}
</Layout>
```

**Функции:**
- Адаптивная боковая панель
- Переключение языка
- Меню пользователя
- Уведомления
- Навигация по разделам

### PrivateRoute

Компонент для защиты маршрутов, требующих авторизации.

```typescript
// Использование
<PrivateRoute requiredRoles={['admin']}>
  <UsersPage />
</PrivateRoute>
```

**Параметры:**
- `requiredRoles` - массив ролей для доступа

### LoadingScreen

Полноэкранный индикатор загрузки.

```typescript
// Использование
if (isLoading) return <LoadingScreen />;
```

## Управление состоянием

### AuthStore

Управление авторизацией и пользователем.

```typescript
interface AuthStore {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  
  // Методы
  login(data: LoginData): Promise<boolean>;
  register(data: RegisterData): Promise<boolean>;
  logout(): void;
  refreshToken(): Promise<boolean>;
  hasRole(roles: string[]): boolean;
}
```

**Использование:**
```typescript
import { authStore } from './stores/AuthStore';

// Вход
const success = await authStore.login({
  login: 'user@example.com',
  password: 'password'
});

// Проверка роли
if (authStore.hasRole(['admin', 'supervisor'])) {
  // Показать админ-функционал
}
```

### PhoneStore

Управление телефонными номерами.

```typescript
interface PhoneStore {
  phones: PhoneNumber[];
  selectedPhone: PhoneNumber | null;
  isLoading: boolean;
  error: string | null;
  
  // Пагинация
  currentPage: number;
  pageSize: number;
  totalItems: number;
  
  // Методы
  fetchPhones(): Promise<void>;
  createPhone(data: PhoneData): Promise<boolean>;
  updatePhone(id: number, data: PhoneData): Promise<boolean>;
  deletePhone(id: number): Promise<boolean>;
  importPhones(file: File): Promise<ImportResult>;
  exportPhones(): Promise<boolean>;
  checkPhone(id: number): Promise<boolean>;
}
```

## Маршрутизация

### Структура маршрутов

```typescript
<Routes>
  {/* Публичные маршруты */}
  <Route path="/login" element={<LoginPage />} />
  <Route path="/register" element={<RegisterPage />} />
  
  {/* Защищенные маршруты */}
  <Route path="/" element={<PrivateRoute><Layout /></PrivateRoute>}>
    <Route index element={<Navigate to="/dashboard" />} />
    <Route path="dashboard" element={<DashboardPage />} />
    <Route path="phones" element={<PhonesPage />} />
    <Route path="checks" element={<ChecksPage />} />
    <Route path="statistics" element={<StatisticsPage />} />
    <Route path="users" element={
      <PrivateRoute requiredRoles={['admin']}>
        <UsersPage />
      </PrivateRoute>
    } />
    <Route path="settings" element={
      <PrivateRoute requiredRoles={['admin', 'supervisor']}>
        <SettingsPage />
      </PrivateRoute>
    } />
  </Route>
</Routes>
```

### Ролевая модель доступа

| Страница | user | supervisor | admin |
|----------|------|------------|-------|
| Dashboard | ✓ | ✓ | ✓ |
| Phones | ✓ | ✓ | ✓ |
| Checks | ✓ | ✓ | ✓ |
| Statistics | ✓ | ✓ | ✓ |
| Users | ✗ | ✗ | ✓ |
| Settings | ✗ | ✓ | ✓ |

## Интернационализация

### Поддерживаемые языки

- 🇬🇧 English (en)
- 🇷🇺 Русский (ru)

### Структура переводов

```typescript
// i18n/locales/en.ts
export const en = {
  common: {
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    // ...
  },
  auth: {
    login: 'Login',
    logout: 'Logout',
    // ...
  },
  // ...
};
```

### Использование

```typescript
import { useTranslation } from 'react-i18next';

function Component() {
  const { t, i18n } = useTranslation();
  
  return (
    <div>
      <h1>{t('dashboard.title')}</h1>
      <Button onClick={() => i18n.changeLanguage('ru')}>
        {t('common.switchLanguage')}
      </Button>
    </div>
  );
}
```

## API интеграция

### Axios конфигурация

```typescript
// Базовый URL
axios.defaults.baseURL = '/api/v1';

// Interceptor для токена
axios.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Interceptor для обновления токена
axios.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      const refreshed = await authStore.refreshToken();
      if (refreshed) {
        return axios(error.config);
      }
      authStore.logout();
    }
    return Promise.reject(error);
  }
);
```

### API эндпоинты

| Метод | Эндпоинт | Описание |
|-------|----------|----------|
| POST | `/auth/login` | Авторизация |
| POST | `/auth/register` | Регистрация |
| POST | `/auth/refresh` | Обновление токена |
| GET | `/phones` | Список номеров |
| POST | `/phones` | Создать номер |
| PUT | `/phones/:id` | Обновить номер |
| DELETE | `/phones/:id` | Удалить номер |
| POST | `/phones/import` | Импорт CSV |
| GET | `/phones/export` | Экспорт CSV |
| POST | `/checks/phone/:id` | Проверить номер |
| POST | `/checks/realtime` | Проверка в реальном времени |
| GET | `/checks/results` | Результаты проверок |
| GET | `/statistics/*` | Статистика |
| GET/POST/PUT/DELETE | `/users/*` | Управление пользователями |
| GET/PUT | `/settings/*` | Настройки системы |

## Темы и стилизация

### Material-UI тема

```typescript
const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: '#90caf9',
    },
    secondary: {
      main: '#f48fb1',
    },
    background: {
      default: '#0a0e1a',
      paper: '#1a1f2e',
    },
  },
  typography: {
    fontFamily: '"Inter", "Roboto", sans-serif',
  },
  shape: {
    borderRadius: 12,
  },
});
```

### CSS утилиты

```css
/* Glassmorphism эффект */
.glass-morphism {
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

/* Градиентный текст */
.gradient-text {
  background: linear-gradient(135deg, #90caf9 0%, #f48fb1 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Анимации */
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
```

## Безопасность

### Защита от XSS

- Все пользовательские данные экранируются React
- Использование `dangerouslySetInnerHTML` запрещено

### Защита маршрутов

- Проверка авторизации через `PrivateRoute`
- Проверка ролей на уровне компонентов
- Перенаправление неавторизованных пользователей

### Хранение токенов

- Access token в localStorage с коротким сроком жизни
- Refresh token для обновления доступа
- Автоматическое обновление токена при 401 ошибке

### Валидация данных

- Валидация форм на клиенте
- Проверка типов через TypeScript
- Обработка ошибок от сервера

## Оптимизация производительности

### Code Splitting

```typescript
// Ленивая загрузка страниц
const DashboardPage = lazy(() => import('./pages/DashboardPage'));
```

### Мемоизация

```typescript
// React.memo для компонентов
const ExpensiveComponent = React.memo(({ data }) => {
  return <div>{/* ... */}</div>;
});

// useMemo для вычислений
const expensiveValue = useMemo(() => {
  return calculateExpensiveValue(data);
}, [data]);
```

### Оптимизация рендеринга

- Использование `observer` из MobX для реактивности
- Виртуализация больших списков через DataGrid
- Дебаунс для поисковых запросов

## Развертывание

### Build для продакшена

```bash
# Создать оптимизированную сборку
npm run build

# Содержимое папки build/ готово для развертывания
```

### Nginx конфигурация

```nginx
server {
    listen 80;
    server_name example.com;
    root /var/www/spamchecker/build;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://backend:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Отладка и логирование

### React Developer Tools

- Установите расширение для браузера
- Инспектируйте компоненты и их состояние
- Профилируйте производительность

### MobX DevTools

```typescript
// Включение логирования MobX
import { configure } from 'mobx';

configure({
  enforceActions: 'always',
  computedRequiresReaction: true,
  reactionRequiresObservable: true,
  observableRequiresReaction: true,
  disableErrorBoundaries: true
});
```

### Обработка ошибок

```typescript
// Error Boundary компонент
class ErrorBoundary extends React.Component {
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Error caught:', error, info);
    // Отправка ошибки в систему мониторинга
  }
}
```

## Тестирование

### Unit тесты

```typescript
// Пример теста компонента
import { render, screen } from '@testing-library/react';
import { LoginPage } from './LoginPage';

test('renders login form', () => {
  render(<LoginPage />);
  expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
  expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
});
```

### E2E тесты

```typescript
// Cypress пример
describe('Login Flow', () => {
  it('should login successfully', () => {
    cy.visit('/login');
    cy.get('[name="login"]').type('user@example.com');
    cy.get('[name="password"]').type('password');
    cy.get('[type="submit"]').click();
    cy.url().should('include', '/dashboard');
  });
});
```

## Поддержка и обслуживание

### Обновление зависимостей

```bash
# Проверка устаревших пакетов
npm outdated

# Обновление пакетов
npm update

# Обновление до последних версий
npm install package@latest
```

### Мониторинг производительности

- Используйте Lighthouse для аудита
- Отслеживайте Core Web Vitals
- Профилируйте bundle size

### Известные проблемы

1. **Safari iOS** - проблемы с backdrop-filter
2. **IE11** - не поддерживается
3. **Старые Android** - требуется полифилл для некоторых функций

## Контакты и поддержка

Для вопросов и предложений обращайтесь к команде разработки.

---

