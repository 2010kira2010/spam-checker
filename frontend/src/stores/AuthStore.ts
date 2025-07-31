import { makeAutoObservable, runInAction } from 'mobx';
import axios from 'axios';

export interface User {
    id: number;
    username: string;
    email: string;
    role: 'admin' | 'supervisor' | 'user';
}

export interface LoginData {
    login: string;
    password: string;
}

export interface AuthTokens {
    access_token: string;
    refresh_token: string;
}

class AuthStore {
    user: User | null = null;
    isAuthenticated = false;
    isLoading = false;
    error: string | null = null;

    constructor() {
        makeAutoObservable(this);
        this.loadFromStorage();
    }

    private loadFromStorage() {
        const accessToken = localStorage.getItem('access_token');
        const userStr = localStorage.getItem('user');

        if (accessToken && userStr) {
            try {
                const user = JSON.parse(userStr);
                runInAction(() => {
                    this.user = user;
                    this.isAuthenticated = true;
                });

                // Set default auth header
                axios.defaults.headers.common['Authorization'] = `Bearer ${accessToken}`;
            } catch (error) {
                this.clearAuth();
            }
        }
    }

    async login(data: LoginData) {
        this.isLoading = true;
        this.error = null;

        try {
            const response = await axios.post<{
                access_token: string;
                refresh_token: string;
                user: User;
            }>('/auth/login', data);

            const { access_token, refresh_token, user } = response.data;

            // Save to localStorage
            localStorage.setItem('access_token', access_token);
            localStorage.setItem('refresh_token', refresh_token);
            localStorage.setItem('user', JSON.stringify(user));

            // Set auth header
            axios.defaults.headers.common['Authorization'] = `Bearer ${access_token}`;

            runInAction(() => {
                this.user = user;
                this.isAuthenticated = true;
                this.isLoading = false;
            });

            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Login failed';
                this.isLoading = false;
            });
            return false;
        }
    }

    async refreshToken() {
        const refreshToken = localStorage.getItem('refresh_token');
        if (!refreshToken) {
            this.clearAuth();
            return false;
        }

        try {
            const response = await axios.post<{ access_token: string }>('/auth/refresh', {
                refresh_token: refreshToken,
            });

            const { access_token } = response.data;

            // Update token
            localStorage.setItem('access_token', access_token);
            axios.defaults.headers.common['Authorization'] = `Bearer ${access_token}`;

            return true;
        } catch (error) {
            this.clearAuth();
            return false;
        }
    }

    logout() {
        this.clearAuth();
        window.location.href = '/login';
    }

    private clearAuth() {
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        localStorage.removeItem('user');
        delete axios.defaults.headers.common['Authorization'];

        runInAction(() => {
            this.user = null;
            this.isAuthenticated = false;
            this.error = null;
        });
    }

    hasRole(roles: string[]) {
        if (!this.user) return false;
        return roles.includes(this.user.role) || this.user.role === 'admin';
    }

    get isAdmin() {
        return this.user?.role === 'admin';
    }

    get isSupervisor() {
        return this.user?.role === 'supervisor' || this.isAdmin;
    }

    clearError() {
        this.error = null;
    }
}

export const authStore = new AuthStore();