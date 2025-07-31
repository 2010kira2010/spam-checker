import { makeAutoObservable, runInAction } from 'mobx';
import axios from 'axios';
import Papa from 'papaparse';

export interface PhoneNumber {
    id: number;
    number: string;
    description: string;
    is_active: boolean;
    created_by: number;
    created_at: string;
    updated_at: string;
    check_results?: CheckResult[];
}

export interface CheckResult {
    id: number;
    service_id: number;
    service: {
        id: number;
        name: string;
        code: string;
    };
    is_spam: boolean;
    found_keywords: string[];
    checked_at: string;
}

export interface PhoneStats {
    total_phones: number;
    active_phones: number;
    spam_phones: number;
    clean_phones: number;
}

class PhoneStore {
    phones: PhoneNumber[] = [];
    selectedPhone: PhoneNumber | null = null;
    stats: PhoneStats | null = null;
    isLoading = false;
    error: string | null = null;

    // Pagination
    currentPage = 1;
    pageSize = 20;
    totalItems = 0;

    // Filters
    searchQuery = '';
    activeFilter: boolean | null = null;

    constructor() {
        makeAutoObservable(this);
    }

    async fetchPhones() {
        this.isLoading = true;
        this.error = null;

        try {
            const params = new URLSearchParams({
                page: this.currentPage.toString(),
                limit: this.pageSize.toString(),
            });

            if (this.searchQuery) {
                params.append('search', this.searchQuery);
            }

            if (this.activeFilter !== null) {
                params.append('is_active', this.activeFilter.toString());
            }

            const response = await axios.get(`/phones?${params}`);

            runInAction(() => {
                this.phones = response.data.phones;
                this.totalItems = response.data.total;
                this.isLoading = false;
            });
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to fetch phones';
                this.isLoading = false;
            });
        }
    }

    async fetchPhoneById(id: number) {
        this.isLoading = true;

        try {
            const response = await axios.get(`/phones/${id}`);

            runInAction(() => {
                this.selectedPhone = response.data;
                this.isLoading = false;
            });
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to fetch phone';
                this.isLoading = false;
            });
        }
    }

    async createPhone(data: { number: string; description: string; is_active: boolean }) {
        this.isLoading = true;
        this.error = null;

        try {
            await axios.post('/phones', data);
            await this.fetchPhones();
            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to create phone';
                this.isLoading = false;
            });
            return false;
        }
    }

    async updatePhone(id: number, data: Partial<PhoneNumber>) {
        this.isLoading = true;
        this.error = null;

        try {
            await axios.put(`/phones/${id}`, data);
            await this.fetchPhones();
            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to update phone';
                this.isLoading = false;
            });
            return false;
        }
    }

    async deletePhone(id: number) {
        this.isLoading = true;
        this.error = null;

        try {
            await axios.delete(`/phones/${id}`);
            await this.fetchPhones();
            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to delete phone';
                this.isLoading = false;
            });
            return false;
        }
    }

    async importPhones(file: File) {
        this.isLoading = true;
        this.error = null;

        const formData = new FormData();
        formData.append('file', file);

        try {
            const response = await axios.post('/phones/import', formData, {
                headers: {
                    'Content-Type': 'multipart/form-data',
                },
            });

            await this.fetchPhones();

            return {
                success: true,
                imported: response.data.imported,
                errors: response.data.errors,
            };
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to import phones';
                this.isLoading = false;
            });
            return {
                success: false,
                imported: 0,
                errors: [this.error],
            };
        }
    }

    async exportPhones() {
        try {
            const params = new URLSearchParams();
            if (this.activeFilter !== null) {
                params.append('is_active', this.activeFilter.toString());
            }

            const response = await axios.get(`/phones/export?${params}`, {
                responseType: 'blob',
            });

            // Download file
            const url = window.URL.createObjectURL(new Blob([response.data]));
            const link = document.createElement('a');
            link.href = url;
            link.setAttribute('download', `phones_${new Date().toISOString().split('T')[0]}.csv`);
            document.body.appendChild(link);
            link.click();
            link.remove();
            window.URL.revokeObjectURL(url);

            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to export phones';
            });
            return false;
        }
    }

    async fetchStats() {
        try {
            const response = await axios.get('/phones/stats');
            runInAction(() => {
                this.stats = response.data;
            });
        } catch (error: any) {
            console.error('Failed to fetch phone stats:', error);
        }
    }

    async checkPhone(id: number) {
        try {
            await axios.post(`/checks/phone/${id}`);
            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to start check';
            });
            return false;
        }
    }

    async checkAllPhones() {
        try {
            await axios.post('/checks/all');
            return true;
        } catch (error: any) {
            runInAction(() => {
                this.error = error.response?.data?.error || 'Failed to start check';
            });
            return false;
        }
    }

    setPage(page: number) {
        this.currentPage = page;
        this.fetchPhones();
    }

    setPageSize(size: number) {
        this.pageSize = size;
        this.currentPage = 1;
        this.fetchPhones();
    }

    setSearchQuery(query: string) {
        this.searchQuery = query;
        this.currentPage = 1;
        this.fetchPhones();
    }

    setActiveFilter(active: boolean | null) {
        this.activeFilter = active;
        this.currentPage = 1;
        this.fetchPhones();
    }

    clearError() {
        this.error = null;
    }

    reset() {
        this.phones = [];
        this.selectedPhone = null;
        this.currentPage = 1;
        this.searchQuery = '';
        this.activeFilter = null;
        this.error = null;
    }
}

export const phoneStore = new PhoneStore();