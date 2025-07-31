import React, { useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { CssBaseline, Box } from '@mui/material';
import { SnackbarProvider } from 'notistack';
import { LocalizationProvider } from '@mui/x-date-pickers';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { observer } from 'mobx-react-lite';
import axios from 'axios';

// Stores
import { authStore } from './stores/AuthStore';

// Components
import Layout from './components/Layout';
import PrivateRoute from './components/PrivateRoute';
import LoadingScreen from './components/LoadingScreen';

// Pages
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import PhonesPage from './pages/PhonesPage';
import ChecksPage from './pages/ChecksPage';
import UsersPage from './pages/UsersPage';
import SettingsPage from './pages/SettingsPage';
import StatisticsPage from './pages/StatisticsPage';
import NotFoundPage from './pages/NotFoundPage';

// Theme configuration
const theme = createTheme({
    palette: {
        mode: 'dark',
        primary: {
            main: '#90caf9',
            light: '#e3f2fd',
            dark: '#42a5f5',
        },
        secondary: {
            main: '#f48fb1',
            light: '#fce4ec',
            dark: '#f06292',
        },
        background: {
            default: '#0a0e1a',
            paper: '#1a1f2e',
        },
        error: {
            main: '#f44336',
        },
        warning: {
            main: '#ffa726',
        },
        info: {
            main: '#29b6f6',
        },
        success: {
            main: '#66bb6a',
        },
    },
    typography: {
        fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
        h1: {
            fontSize: '2.5rem',
            fontWeight: 600,
        },
        h2: {
            fontSize: '2rem',
            fontWeight: 600,
        },
        h3: {
            fontSize: '1.75rem',
            fontWeight: 600,
        },
        h4: {
            fontSize: '1.5rem',
            fontWeight: 500,
        },
        h5: {
            fontSize: '1.25rem',
            fontWeight: 500,
        },
        h6: {
            fontSize: '1rem',
            fontWeight: 500,
        },
    },
    shape: {
        borderRadius: 12,
    },
    components: {
        MuiButton: {
            styleOverrides: {
                root: {
                    textTransform: 'none',
                    fontWeight: 500,
                    borderRadius: 8,
                },
            },
        },
        MuiCard: {
            styleOverrides: {
                root: {
                    backgroundImage: 'none',
                    backgroundColor: '#1a1f2e',
                    borderRadius: 16,
                },
            },
        },
        MuiPaper: {
            styleOverrides: {
                root: {
                    backgroundImage: 'none',
                    backgroundColor: '#1a1f2e',
                },
            },
        },
    },
});

// Axios configuration
axios.defaults.baseURL = '/api/v1';

// Request interceptor
axios.interceptors.request.use(
    (config) => {
        const token = localStorage.getItem('access_token');
        if (token) {
            config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
    },
    (error) => {
        return Promise.reject(error);
    }
);

// Response interceptor
axios.interceptors.response.use(
    (response) => response,
    async (error) => {
        const originalRequest = error.config;

        if (error.response?.status === 401 && !originalRequest._retry) {
            originalRequest._retry = true;

            const refreshed = await authStore.refreshToken();
            if (refreshed) {
                return axios(originalRequest);
            } else {
                authStore.logout();
            }
        }

        return Promise.reject(error);
    }
);

const App: React.FC = observer(() => {
    const [isInitialized, setIsInitialized] = React.useState(false);

    useEffect(() => {
        // Initialize app
        setIsInitialized(true);
    }, []);

    if (!isInitialized) {
        return <LoadingScreen />;
    }

    return (
        <ThemeProvider theme={theme}>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
                <SnackbarProvider
                    maxSnack={3}
                    anchorOrigin={{
                        vertical: 'bottom',
                        horizontal: 'right',
                    }}
                    autoHideDuration={4000}
                >
                    <CssBaseline />
                    <Router>
                        <Routes>
                            {/* Public routes */}
                            <Route path="/login" element={authStore.isAuthenticated ? <Navigate to="/" /> : <LoginPage />} />

                            {/* Private routes */}
                            <Route
                                path="/"
                                element={
                                    <PrivateRoute>
                                        <Layout />
                                    </PrivateRoute>
                                }
                            >
                                <Route index element={<Navigate to="/dashboard" />} />
                                <Route path="dashboard" element={<DashboardPage />} />
                                <Route path="phones" element={<PhonesPage />} />
                                <Route path="checks" element={<ChecksPage />} />
                                <Route path="statistics" element={<StatisticsPage />} />
                                <Route
                                    path="users"
                                    element={
                                        <PrivateRoute requiredRoles={['admin']}>
                                            <UsersPage />
                                        </PrivateRoute>
                                    }
                                />
                                <Route
                                    path="settings"
                                    element={
                                        <PrivateRoute requiredRoles={['admin', 'supervisor']}>
                                            <SettingsPage />
                                        </PrivateRoute>
                                    }
                                />
                                <Route path="*" element={<NotFoundPage />} />
                            </Route>
                        </Routes>
                    </Router>
                </SnackbarProvider>
            </LocalizationProvider>
        </ThemeProvider>
    );
});

export default App;