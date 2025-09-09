import React, { useState, useEffect } from 'react';
import { observer } from 'mobx-react-lite';
import { useTranslation } from 'react-i18next';
import { mdiDocker } from '@mdi/js';
import {
    Box,
    Card,
    CardContent,
    Typography,
    Tabs,
    Tab,
    TextField,
    Button,
    Switch,
    FormControlLabel,
    Grid,
    Alert,
    IconButton,
    List,
    ListItem,
    ListItemText,
    ListItemSecondaryAction,
    Chip,
    Paper,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Select,
    MenuItem,
    FormControl,
    InputLabel,
    Tooltip,
    CircularProgress,
    Radio,
    RadioGroup,
    Divider,
    SvgIcon,
    SvgIconProps,
    LinearProgress,
    Collapse,
    InputAdornment,
} from '@mui/material';
import {
    Settings as SettingsIcon,
    Android,
    Notifications,
    Schedule,
    TextFields,
    Storage,
    Scanner,
    Save,
    Add,
    Edit,
    Delete,
    WifiTethering,
    Refresh,
    Language,
    Computer,
    CloudUpload,
    OpenInNew,
    Api,
    PlayArrow,
    Code,
    ExpandMore,
    ExpandLess,
    ContentCopy,
    Phone,
    CheckCircle as CheckCircleIcon,
    Error as ErrorIcon,
    Info as InfoIcon,
} from '@mui/icons-material';
import { useSnackbar } from 'notistack';
import axios from 'axios';

interface GeneralSettings {
    check_interval_minutes: number;
    max_concurrent_checks: number;
    notification_batch_size: number;
    screenshot_quality: number;
    ocr_confidence_threshold: number;
    check_mode: string;
}

interface ADBGateway {
    id: number;
    name: string;
    host: string;
    port: number;
    service_code: string;
    is_active: boolean;
    status: string;
    device_id?: string;
    last_ping?: string;
    is_docker: boolean;
    container_id?: string;
    vnc_port?: number;
    adb_port1?: number;
    adb_port2?: number;
}

interface APIService {
    id: number;
    name: string;
    service_code: string;
    api_url: string;
    headers: string;
    method: string;
    request_body?: string;
    timeout: number;
    is_active: boolean;
    response_path?: string;
    keyword_paths?: string;
}

interface SpamKeyword {
    id: number;
    keyword: string;
    service_id?: number;
    is_active: boolean;
}

interface CheckSchedule {
    id: number;
    name: string;
    cron_expression: string;
    is_active: boolean;
    last_run?: string;
    next_run?: string;
}

interface Notification {
    id: number;
    type: string;
    config: any;
    is_active: boolean;
}

interface TabPanelProps {
    children?: React.ReactNode;
    index: number;
    value: number;
}

function TabPanel(props: TabPanelProps) {
    const { children, value, index, ...other } = props;

    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`settings-tabpanel-${index}`}
            aria-labelledby={`settings-tab-${index}`}
            {...other}
        >
            {value === index && <Box sx={{ py: 3 }}>{children}</Box>}
        </div>
    );
}

function DockerIcon(props: SvgIconProps) {
    return (
        <SvgIcon {...props}>
            <path d={mdiDocker} />
        </SvgIcon>
    );
}

const SettingsPage: React.FC = observer(() => {
    const { t, i18n } = useTranslation();
    const { enqueueSnackbar } = useSnackbar();
    const [tabValue, setTabValue] = useState(0);
    const [isLoading, setIsLoading] = useState(false);

    // General settings
    const [generalSettings, setGeneralSettings] = useState<GeneralSettings>({
        check_interval_minutes: 60,
        max_concurrent_checks: 3,
        notification_batch_size: 50,
        screenshot_quality: 80,
        ocr_confidence_threshold: 70,
        check_mode: 'adb_only',
    });

    // ADB Gateways
    const [adbGateways, setAdbGateways] = useState<ADBGateway[]>([]);
    const [adbDialogOpen, setAdbDialogOpen] = useState(false);
    const [editingGateway, setEditingGateway] = useState<ADBGateway | null>(null);
    const [gatewayCreationType, setGatewayCreationType] = useState<'manual' | 'docker'>('manual');
    const [dockerAPKFile, setDockerAPKFile] = useState<File | null>(null);

    // API Services
    const [apiServices, setApiServices] = useState<APIService[]>([]);
    const [apiDialogOpen, setApiDialogOpen] = useState(false);
    const [editingApiService, setEditingApiService] = useState<APIService | null>(null);
    const [testingApiService, setTestingApiService] = useState<number | null>(null);
    const [testPhoneNumber, setTestPhoneNumber] = useState('');
    const [testResults, setTestResults] = useState<Record<number, any>>({});
    const [headersEditorOpen, setHeadersEditorOpen] = useState(false);
    const [editingHeaders, setEditingHeaders] = useState('');
    const [expandedApiService, setExpandedApiService] = useState<number | null>(null);

    // Keywords
    const [keywords, setKeywords] = useState<SpamKeyword[]>([]);
    const [keywordDialogOpen, setKeywordDialogOpen] = useState(false);
    const [editingKeyword, setEditingKeyword] = useState<SpamKeyword | null>(null);

    // Schedules
    const [schedules, setSchedules] = useState<CheckSchedule[]>([]);
    const [scheduleDialogOpen, setScheduleDialogOpen] = useState(false);
    const [editingSchedule, setEditingSchedule] = useState<CheckSchedule | null>(null);

    // Notifications
    const [notifications, setNotifications] = useState<Notification[]>([]);
    const [notificationDialogOpen, setNotificationDialogOpen] = useState(false);
    const [editingNotification, setEditingNotification] = useState<Notification | null>(null);

    useEffect(() => {
        loadSettings();
    }, []);

    const loadSettings = async () => {
        setIsLoading(true);
        try {
            // Load all settings
            const [settingsRes, gatewaysRes, apisRes, keywordsRes, schedulesRes, notificationsRes] = await Promise.all([
                axios.get('/settings'),
                axios.get('/adb/gateways'),
                axios.get('/api-services').catch(() => ({ data: [] })),
                axios.get('/settings/keywords'),
                axios.get('/settings/schedules'),
                axios.get('/notifications'),
            ]);

            // Parse general settings
            const settings: any = {};
            settingsRes.data.forEach((setting: any) => {
                if (setting.type === 'int') {
                    settings[setting.key] = parseInt(setting.value);
                } else {
                    settings[setting.key] = setting.value;
                }
            });
            setGeneralSettings({
                ...generalSettings,
                ...settings,
            });

            setAdbGateways(gatewaysRes.data);
            setApiServices(apisRes.data);
            setKeywords(keywordsRes.data);
            setSchedules(schedulesRes.data);
            setNotifications(notificationsRes.data);
        } catch (error) {
            enqueueSnackbar(t('errors.loadFailed'), { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
        setTabValue(newValue);
    };

    // API Service handlers
    const handleAddApiService = () => {
        setEditingApiService({
            id: 0,
            name: '',
            service_code: 'custom_api',
            api_url: '',
            headers: '{}',
            method: 'GET',
            request_body: '',
            timeout: 30,
            is_active: true,
            response_path: '',
            keyword_paths: '',
        });
        setApiDialogOpen(true);
    };

    const handleEditApiService = (service: APIService) => {
        setEditingApiService(service);
        setApiDialogOpen(true);
    };

    const handleSaveApiService = async () => {
        if (!editingApiService) return;

        // Validation
        if (!editingApiService.name.trim()) {
            enqueueSnackbar('Name is required', { variant: 'error' });
            return;
        }

        if (!editingApiService.api_url.trim()) {
            enqueueSnackbar('API URL is required', { variant: 'error' });
            return;
        }

        try {
            new URL(editingApiService.api_url);
        } catch {
            enqueueSnackbar('Invalid API URL format', { variant: 'error' });
            return;
        }

        // Validate headers JSON
        if (editingApiService.headers) {
            try {
                JSON.parse(editingApiService.headers);
            } catch {
                enqueueSnackbar('Invalid headers JSON format', { variant: 'error' });
                return;
            }
        }

        // Validate request body JSON for non-GET methods
        if (editingApiService.method !== 'GET' && editingApiService.request_body) {
            try {
                JSON.parse(editingApiService.request_body);
            } catch {
                enqueueSnackbar('Invalid request body JSON format', { variant: 'error' });
                return;
            }
        }

        try {
            if (editingApiService.id === 0) {
                const res = await axios.post('/api-services', editingApiService);
                setApiServices([...apiServices, res.data]);
            } else {
                await axios.put(`/api-services/${editingApiService.id}`, editingApiService);
                setApiServices(apiServices.map(s => s.id === editingApiService.id ? editingApiService : s));
            }
            setApiDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error: any) {
            enqueueSnackbar(error.response?.data?.error || t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleDeleteApiService = async (id: number) => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) return;

        try {
            await axios.delete(`/api-services/${id}`);
            setApiServices(apiServices.filter(s => s.id !== id));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    const handleTestApiService = async (id: number) => {
        if (!testPhoneNumber) {
            enqueueSnackbar('Please enter a phone number to test', { variant: 'warning' });
            return;
        }

        setTestingApiService(id);
        setTestResults(prev => ({ ...prev, [id]: null }));

        try {
            const res = await axios.post(`/api-services/${id}/test`, {
                phone_number: testPhoneNumber,
            });
            setTestResults(prev => ({ ...prev, [id]: res.data }));
            enqueueSnackbar('Test completed', { variant: 'success' });
        } catch (error: any) {
            enqueueSnackbar(error.response?.data?.error || 'Test failed', { variant: 'error' });
            setTestResults(prev => ({ ...prev, [id]: { error: error.response?.data?.error || 'Test failed' } }));
        } finally {
            setTestingApiService(null);
        }
    };

    const handleToggleApiService = async (service: APIService) => {
        try {
            await axios.post(`/api-services/${service.id}/toggle`);
            setApiServices(apiServices.map(s =>
                s.id === service.id ? { ...s, is_active: !s.is_active } : s
            ));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.updateFailed'), { variant: 'error' });
        }
    };

    const handleOpenHeadersEditor = (headers: string) => {
        try {
            const parsed = JSON.parse(headers || '{}');
            setEditingHeaders(JSON.stringify(parsed, null, 2));
        } catch {
            setEditingHeaders('{}');
        }
        setHeadersEditorOpen(true);
    };

    const handleSaveHeaders = () => {
        try {
            JSON.parse(editingHeaders);
            if (editingApiService) {
                setEditingApiService({ ...editingApiService, headers: editingHeaders });
            }
            setHeadersEditorOpen(false);
        } catch {
            enqueueSnackbar('Invalid JSON format', { variant: 'error' });
        }
    };

    const handleCopyExample = (example: any) => {
        const formatted = JSON.stringify(example, null, 2);
        navigator.clipboard.writeText(formatted);
        enqueueSnackbar('Copied to clipboard', { variant: 'success' });
    };

    const exampleConfigs = {
        yandex: {
            headers: {
                "App-Build-Number": "257",
                "App-Version-Name": "25.7",
                "Accept-Language": "ru",
                "OS-Name": "iOS",
                "OS-Version": "18.5",
                "X-Src": "ru.yandex.mobile.search",
                "User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X)",
            },
            response_path: "$.items[?(@.id=='caller-id')].snippet",
            keyword_paths: "$.items[*].snippet"
        },
        truecaller: {
            headers: {
                "Accept": "application/json",
                "Authorization": "Bearer YOUR_API_KEY",
                "User-Agent": "Truecaller/12.0.0"
            },
            response_path: "$.data.name",
            keyword_paths: "$.data.tags[*]"
        },
        getcontact: {
            headers: {
                "X-Api-Version": "2.0",
                "X-Client-Version": "4.9.1",
                "Accept": "application/json"
            },
            response_path: "$.result.display_name",
            keyword_paths: "$.result.tags[*].tag"
        }
    };

    const handleSaveGateway = async () => {
        if (!editingGateway) return;

        try {
            if (editingGateway.id === 0) {
                if (gatewayCreationType === 'docker') {
                    // Create Docker gateway
                    const formData = new FormData();
                    formData.append('name', editingGateway.name);
                    formData.append('service_code', editingGateway.service_code);
                    if (dockerAPKFile) {
                        formData.append('apk', dockerAPKFile);
                    }

                    const res = await axios.post('/adb/gateways/docker', formData, {
                        headers: {
                            'Content-Type': 'multipart/form-data',
                        },
                    });
                    setAdbGateways([...adbGateways, res.data]);
                    enqueueSnackbar('Docker gateway creation started. It may take a few minutes to be ready.', { variant: 'info' });
                } else {
                    // Create manual gateway
                    const res = await axios.post('/adb/gateways', editingGateway);
                    setAdbGateways([...adbGateways, res.data]);
                }
            } else {
                await axios.put(`/adb/gateways/${editingGateway.id}`, editingGateway);
                setAdbGateways(adbGateways.map(g => g.id === editingGateway.id ? editingGateway : g));
            }
            setAdbDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });

            // Reload gateways after a delay for Docker containers
            if (gatewayCreationType === 'docker') {
                setTimeout(() => {
                    loadSettings();
                }, 5000);
            }
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleDeleteGateway = async (id: number) => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) return;

        try {
            await axios.delete(`/adb/gateways/${id}`);
            setAdbGateways(adbGateways.filter(g => g.id !== id));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    const handleUpdateGatewayStatus = async (id: number) => {
        try {
            await axios.post(`/adb/gateways/${id}/status`);
            loadSettings();
        } catch (error) {
            enqueueSnackbar(t('errors.updateFailed'), { variant: 'error' });
        }
    };

    const handleOpenVNC = (gateway: ADBGateway) => {
        if (gateway.vnc_port) {
            window.open(`http://localhost:${gateway.vnc_port}`, '_blank');
        }
    };

    // Keyword handlers
    const handleAddKeyword = () => {
        setEditingKeyword({
            id: 0,
            keyword: '',
            is_active: true,
        });
        setKeywordDialogOpen(true);
    };

    const handleEditKeyword = (keyword: SpamKeyword) => {
        setEditingKeyword(keyword);
        setKeywordDialogOpen(true);
    };

    const handleSaveKeyword = async () => {
        if (!editingKeyword) return;

        try {
            if (editingKeyword.id === 0) {
                const res = await axios.post('/settings/keywords', editingKeyword);
                setKeywords([...keywords, res.data]);
            } else {
                await axios.put(`/settings/keywords/${editingKeyword.id}`, editingKeyword);
                setKeywords(keywords.map(k => k.id === editingKeyword.id ? editingKeyword : k));
            }
            setKeywordDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleDeleteKeyword = async (id: number) => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) return;

        try {
            await axios.delete(`/settings/keywords/${id}`);
            setKeywords(keywords.filter(k => k.id !== id));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    // Schedule handlers
    const handleAddSchedule = () => {
        setEditingSchedule({
            id: 0,
            name: '',
            cron_expression: '@hourly',
            is_active: true,
        });
        setScheduleDialogOpen(true);
    };

    const handleEditSchedule = (schedule: CheckSchedule) => {
        setEditingSchedule(schedule);
        setScheduleDialogOpen(true);
    };

    const handleSaveSchedule = async () => {
        if (!editingSchedule) return;

        try {
            if (editingSchedule.id === 0) {
                const res = await axios.post('/settings/schedules', editingSchedule);
                setSchedules([...schedules, res.data]);
            } else {
                await axios.put(`/settings/schedules/${editingSchedule.id}`, editingSchedule);
                setSchedules(schedules.map(s => s.id === editingSchedule.id ? editingSchedule : s));
            }
            setScheduleDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleDeleteSchedule = async (id: number) => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) return;

        try {
            await axios.delete(`/settings/schedules/${id}`);
            setSchedules(schedules.filter(s => s.id !== id));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    const handleToggleSchedule = async (schedule: CheckSchedule, isActive: boolean) => {
        try {
            const updated = { ...schedule, is_active: isActive };
            await axios.put(`/settings/schedules/${schedule.id}`, updated);
            setSchedules(schedules.map(s => s.id === schedule.id ? updated : s));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    // Notification handlers
    const handleAddNotification = () => {
        setEditingNotification({
            id: 0,
            type: 'telegram',
            config: {},
            is_active: true,
        });
        setNotificationDialogOpen(true);
    };

    const handleEditNotification = (notification: Notification) => {
        setEditingNotification(notification);
        setNotificationDialogOpen(true);
    };

    const handleSaveNotification = async () => {
        if (!editingNotification) return;

        try {
            if (editingNotification.id === 0) {
                const res = await axios.post('/notifications', {
                    ...editingNotification,
                    config: JSON.stringify(editingNotification.config),
                });
                setNotifications([...notifications, res.data]);
            } else {
                await axios.put(`/notifications/${editingNotification.id}`, {
                    ...editingNotification,
                    config: JSON.stringify(editingNotification.config),
                });
                setNotifications(notifications.map(n => n.id === editingNotification.id ? editingNotification : n));
            }
            setNotificationDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleDeleteNotification = async (id: number) => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) return;

        try {
            await axios.delete(`/notifications/${id}`);
            setNotifications(notifications.filter(n => n.id !== id));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    const handleToggleNotification = async (notification: Notification, isActive: boolean) => {
        try {
            await axios.put(`/notifications/${notification.id}`, {
                ...notification,
                is_active: isActive,
                config: JSON.stringify(notification.config),
            });
            setNotifications(notifications.map(n => n.id === notification.id ? { ...n, is_active: isActive } : n));
            enqueueSnackbar(t('common.success'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleTestNotification = async (id: number) => {
        try {
            await axios.post(`/notifications/${id}/test`);
            enqueueSnackbar(t('notifications.testSent'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.error'), { variant: 'error' });
        }
    };

    const getStatusColor = (status: string) => {
        switch (status) {
            case 'online':
                return 'success';
            case 'offline':
                return 'error';
            case 'restarting':
                return 'warning';
            case 'creating':
                return 'info';
            default:
                return 'default';
        }
    };

    const getNotificationConfig = (notification: Notification) => {
        if (typeof notification.config === 'string') {
            try {
                return JSON.parse(notification.config);
            } catch {
                return {};
            }
        }
        return notification.config;
    };

    const handleSaveGeneralSettings = async () => {
        try {
            const updates = Object.entries(generalSettings).map(([key, value]) => ({
                key,
                value: value.toString(),
            }));

            await Promise.all(updates.map(update =>
                axios.put(`/settings/${update.key}`, { value: update.value })
            ));

            enqueueSnackbar(t('settings.settingsSaved'), { variant: 'success' });
        } catch (error) {
            enqueueSnackbar(t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const changeLanguage = (lng: string) => {
        i18n.changeLanguage(lng);
    };

    // ADB Gateway handlers
    const handleAddGateway = () => {
        setEditingGateway({
            id: 0,
            name: '',
            host: '',
            port: 5554,
            service_code: 'yandex_aon',
            is_active: true,
            status: 'offline',
            is_docker: false,
        });
        setGatewayCreationType('manual');
        setDockerAPKFile(null);
        setAdbDialogOpen(true);
    };

    const handleEditGateway = (gateway: ADBGateway) => {
        setEditingGateway(gateway);
        setGatewayCreationType(gateway.is_docker ? 'docker' : 'manual');
        setAdbDialogOpen(true);
    };

    const formatJson = (jsonString: string) => {
        try {
            const parsed = JSON.parse(jsonString);
            return JSON.stringify(parsed, null, 2);
        } catch {
            return jsonString;
        }
    };

    const validateJson = (jsonString: string) => {
        try {
            JSON.parse(jsonString);
            return true;
        } catch {
            return false;
        }
    };

    return (
        <Box>
            {isLoading ? (
                <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
                    <CircularProgress />
                </Box>
            ) : (
                <Card>
                    <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                        <Tabs value={tabValue} onChange={handleTabChange} variant="scrollable" scrollButtons="auto">
                            <Tab icon={<SettingsIcon />} label={t('settings.general')} />
                            <Tab icon={<Android />} label={t('settings.adbGateways')} />
                            <Tab icon={<Api />} label={t('settings.apiServices')} />
                            <Tab icon={<Scanner />} label={t('settings.ocrSettings')} />
                            <Tab icon={<TextFields />} label={t('settings.keywords')} />
                            <Tab icon={<Schedule />} label={t('settings.schedules')} />
                            <Tab icon={<Notifications />} label={t('settings.notifications')} />
                            <Tab icon={<Storage />} label={t('settings.database')} />
                        </Tabs>
                    </Box>

                    <CardContent>
                        {/* General Settings */}
                        <TabPanel value={tabValue} index={0}>
                            <Grid container spacing={3}>
                                <Grid item xs={12}>
                                    <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.checkMode')}</Typography>
                                    <FormControl component="fieldset">
                                        <RadioGroup
                                            value={generalSettings.check_mode}
                                            onChange={(e) => setGeneralSettings({ ...generalSettings, check_mode: e.target.value })}
                                        >
                                            <FormControlLabel
                                                value="adb_only"
                                                control={<Radio />}
                                                label={t('settings.checkModeADBonly')}
                                            />
                                            <FormControlLabel
                                                value="api_only"
                                                control={<Radio />}
                                                label={t('settings.checkModeAPIonly')}
                                            />
                                            <FormControlLabel
                                                value="both"
                                                control={<Radio />}
                                                label={t('settings.checkModeBoth')}
                                            />
                                        </RadioGroup>
                                    </FormControl>
                                </Grid>
                                <Grid item xs={12}>
                                    <Divider sx={{ my: 2 }} />
                                </Grid>
                                <Grid item xs={12} md={6}>
                                    <TextField
                                        fullWidth
                                        label={t('settings.checkInterval')}
                                        type="number"
                                        value={generalSettings.check_interval_minutes}
                                        onChange={(e) => setGeneralSettings({ ...generalSettings, check_interval_minutes: parseInt(e.target.value) })}
                                        helperText={t('settings.checkIntervalHelp')}
                                    />
                                </Grid>
                                <Grid item xs={12} md={6}>
                                    <TextField
                                        fullWidth
                                        label={t('settings.maxConcurrentChecks')}
                                        type="number"
                                        value={generalSettings.max_concurrent_checks}
                                        onChange={(e) => setGeneralSettings({ ...generalSettings, max_concurrent_checks: parseInt(e.target.value) })}
                                        helperText={t('settings.maxConcurrentChecksHelp')}
                                    />
                                </Grid>
                                <Grid item xs={12} md={6}>
                                    <TextField
                                        fullWidth
                                        label={t('settings.notificationBatchSize')}
                                        type="number"
                                        value={generalSettings.notification_batch_size}
                                        onChange={(e) => setGeneralSettings({ ...generalSettings, notification_batch_size: parseInt(e.target.value) })}
                                        helperText={t('settings.notificationBatchSizeHelp')}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <Button variant="contained" startIcon={<Save />} onClick={handleSaveGeneralSettings}>
                                        {t('settings.saveSettings')}
                                    </Button>
                                </Grid>
                            </Grid>
                        </TabPanel>

                        {/* ADB Gateways */}
                        <TabPanel value={tabValue} index={1}>
                            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Typography variant="h6">{t('settings.androidDebugBridgeGateways')}</Typography>
                                <Button variant="contained" startIcon={<Add />} onClick={handleAddGateway}>
                                    {t('settings.addGateway')}
                                </Button>
                            </Box>
                            <List>
                                {adbGateways.map((gateway) => (
                                    <Paper key={gateway.id} sx={{ mb: 2, p: 2 }}>
                                        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                                <WifiTethering color={gateway.status === 'online' ? 'success' : 'error'} />
                                                <Box>
                                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                        <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                                                            {gateway.name}
                                                        </Typography>
                                                        {gateway.is_docker && (
                                                            <Chip
                                                                icon={<DockerIcon/>}
                                                                label="Docker"
                                                                size="small"
                                                                color="primary"
                                                                variant="outlined"
                                                            />
                                                        )}
                                                    </Box>
                                                    <Typography variant="body2" color="text.secondary">
                                                        {gateway.host}:{gateway.port} • {t('settings.serviceCode')}: {gateway.service_code}
                                                    </Typography>
                                                    {gateway.is_docker && gateway.vnc_port && (
                                                        <Typography variant="caption" color="text.secondary">
                                                            VNC Port: {gateway.vnc_port} • ADB Ports: {gateway.adb_port1}, {gateway.adb_port2}
                                                        </Typography>
                                                    )}
                                                </Box>
                                                <Chip
                                                    label={t(`settings.${gateway.status}`)}
                                                    size="small"
                                                    color={getStatusColor(gateway.status)}
                                                />
                                            </Box>
                                            <Box>
                                                {gateway.is_docker && gateway.vnc_port && gateway.status === 'online' && (
                                                    <Tooltip title="Open VNC">
                                                        <IconButton size="small" onClick={() => handleOpenVNC(gateway)}>
                                                            <OpenInNew />
                                                        </IconButton>
                                                    </Tooltip>
                                                )}
                                                <Tooltip title={t('common.refresh')}>
                                                    <IconButton size="small" onClick={() => handleUpdateGatewayStatus(gateway.id)}>
                                                        <Refresh />
                                                    </IconButton>
                                                </Tooltip>
                                                <IconButton size="small" onClick={() => handleEditGateway(gateway)}>
                                                    <Edit />
                                                </IconButton>
                                                <IconButton size="small" color="error" onClick={() => handleDeleteGateway(gateway.id)}>
                                                    <Delete />
                                                </IconButton>
                                            </Box>
                                        </Box>
                                    </Paper>
                                ))}
                            </List>
                        </TabPanel>

                        {/* API Services */}
                        <TabPanel value={tabValue} index={2}>
                            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Typography variant="h6">{t('settings.apiServices')}</Typography>
                                <Button variant="contained" startIcon={<Add />} onClick={handleAddApiService}>
                                    {t('settings.apiServicesAdd')}
                                </Button>
                            </Box>

                            <Alert severity="info" sx={{ mb: 3 }}>
                                {t('settings.apiServicesInformer')}
                            </Alert>

                            <Box sx={{ mb: 3 }}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServicesTestPhone')}
                                    placeholder="+7 (999) 123-45-67"
                                    value={testPhoneNumber}
                                    onChange={(e) => setTestPhoneNumber(e.target.value)}
                                    helperText={t('settings.apiServicesHelperText')}
                                    InputProps={{
                                        startAdornment: (
                                            <InputAdornment position="start">
                                                <Phone />
                                            </InputAdornment>
                                        ),
                                    }}
                                />
                            </Box>

                            <List>
                                {apiServices.map((service) => (
                                    <Paper key={service.id} sx={{ mb: 2 }}>
                                        <Box sx={{ p: 2 }}>
                                            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flex: 1 }}>
                                                    <Api color={service.is_active ? 'success' : 'disabled'} />
                                                    <Box sx={{ flex: 1 }}>
                                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                            <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                                                                {service.name}
                                                            </Typography>
                                                            <Chip
                                                                label={service.method}
                                                                size="small"
                                                                color="primary"
                                                                variant="outlined"
                                                            />
                                                            <Chip
                                                                label={service.service_code}
                                                                size="small"
                                                                variant="outlined"
                                                            />
                                                            <Chip
                                                                label={service.is_active ? t('settings.apiServiceActive') : t('settings.apiServiceInactive')}
                                                                size="small"
                                                                color={service.is_active ? 'success' : 'default'}
                                                            />
                                                        </Box>
                                                        <Typography variant="body2" color="text.secondary" sx={{
                                                            whiteSpace: 'nowrap',
                                                            overflow: 'hidden',
                                                            textOverflow: 'ellipsis',
                                                            maxWidth: '600px'
                                                        }}>
                                                            {service.api_url}
                                                        </Typography>
                                                        <Typography variant="caption" color="text.secondary">
                                                            {t('settings.apiServiceTimeout', { seconds: service.timeout })}
                                                        </Typography>
                                                    </Box>
                                                </Box>
                                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                    <Tooltip title={t('settings.apiServiceExpandDetails')}>
                                                        <IconButton
                                                            size="small"
                                                            onClick={() => setExpandedApiService(expandedApiService === service.id ? null : service.id)}
                                                        >
                                                            {expandedApiService === service.id ? <ExpandLess /> : <ExpandMore />}
                                                        </IconButton>
                                                    </Tooltip>
                                                    <Tooltip title={t('settings.apiServiceTest')}>
                                                        <IconButton
                                                            size="small"
                                                            onClick={() => handleTestApiService(service.id)}
                                                            disabled={testingApiService === service.id || !testPhoneNumber}
                                                        >
                                                            {testingApiService === service.id ? <CircularProgress size={20} /> : <PlayArrow />}
                                                        </IconButton>
                                                    </Tooltip>
                                                    <Tooltip title={t('settings.apiServiceToggleActive')}>
                                                        <IconButton
                                                            size="small"
                                                            onClick={() => handleToggleApiService(service)}
                                                        >
                                                            <Switch checked={service.is_active} />
                                                        </IconButton>
                                                    </Tooltip>
                                                    <IconButton size="small" onClick={() => handleEditApiService(service)}>
                                                        <Edit />
                                                    </IconButton>
                                                    <IconButton size="small" color="error" onClick={() => handleDeleteApiService(service.id)}>
                                                        <Delete />
                                                    </IconButton>
                                                </Box>
                                            </Box>

                                            <Collapse in={expandedApiService === service.id}>
                                                <Box sx={{ mt: 2, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
                                                    <Grid container spacing={2}>
                                                        <Grid item xs={12} md={6}>
                                                            <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceHeaders')}</Typography>
                                                            <pre style={{
                                                                margin: 0,
                                                                fontSize: '11px',
                                                                overflow: 'auto',
                                                                maxHeight: '150px',
                                                                backgroundColor: 'background.paper',
                                                                padding: '8px',
                                                                borderRadius: '4px'
                                                            }}>
                                        {formatJson(service.headers)}
                                    </pre>
                                                        </Grid>
                                                        {service.method !== 'GET' && service.request_body && (
                                                            <Grid item xs={12} md={6}>
                                                                <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceRequestBody')}</Typography>
                                                                <pre style={{
                                                                    margin: 0,
                                                                    fontSize: '11px',
                                                                    overflow: 'auto',
                                                                    maxHeight: '150px',
                                                                    backgroundColor: 'background.paper',
                                                                    padding: '8px',
                                                                    borderRadius: '4px'
                                                                }}>
                                            {formatJson(service.request_body || '{}')}
                                        </pre>
                                                            </Grid>
                                                        )}
                                                        {service.response_path && (
                                                            <Grid item xs={12}>
                                                                <Typography variant="caption" color="text.secondary">
                                                                    {t('settings.apiServiceResponsePath')} <code>{service.response_path}</code>
                                                                </Typography>
                                                            </Grid>
                                                        )}
                                                        {service.keyword_paths && (
                                                            <Grid item xs={12}>
                                                                <Typography variant="caption" color="text.secondary">
                                                                    {t('settings.apiServiceKeywordPaths')} <code>{service.keyword_paths}</code>
                                                                </Typography>
                                                            </Grid>
                                                        )}
                                                    </Grid>
                                                </Box>
                                            </Collapse>

                                            {testResults[service.id] && (
                                                <Box sx={{ mt: 2, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
                                                    <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceTestResults')}</Typography>
                                                    {testResults[service.id].error ? (
                                                        <Alert severity="error" icon={<ErrorIcon />}>{testResults[service.id].error}</Alert>
                                                    ) : (
                                                        <>
                                                            <Box sx={{ display: 'flex', gap: 2, mb: 1, flexWrap: 'wrap' }}>
                                                                <Chip
                                                                    label={t('settings.apiServiceStatus', { code: testResults[service.id].status_code })}
                                                                    color={testResults[service.id].status_code === 200 ? 'success' : 'error'}
                                                                    size="small"
                                                                    icon={testResults[service.id].status_code === 200 ? <CheckCircleIcon /> : <ErrorIcon />}
                                                                />
                                                                <Chip
                                                                    label={t('settings.apiServiceResponseTime', { time: testResults[service.id].response_time })}
                                                                    size="small"
                                                                    variant="outlined"
                                                                />
                                                                <Chip
                                                                    label={testResults[service.id].is_spam ? t('settings.apiServiceSpamDetected') : t('settings.apiServiceClean')}
                                                                    color={testResults[service.id].is_spam ? 'error' : 'success'}
                                                                    size="small"
                                                                />
                                                            </Box>
                                                            {testResults[service.id].keywords && testResults[service.id].keywords.length > 0 && (
                                                                <Box sx={{ mb: 1 }}>
                                                                    <Typography variant="caption">{t('settings.apiServiceKeywordsFound')} </Typography>
                                                                    {testResults[service.id].keywords.map((kw: string, idx: number) => (
                                                                        <Chip key={idx} label={kw} size="small" color="error" sx={{ ml: 0.5 }} />
                                                                    ))}
                                                                </Box>
                                                            )}
                                                            <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceResponse')}</Typography>
                                                            <pre style={{
                                                                margin: 0,
                                                                fontSize: '12px',
                                                                whiteSpace: 'pre-wrap',
                                                                wordBreak: 'break-word',
                                                                maxHeight: '200px',
                                                                overflow: 'auto',
                                                                backgroundColor: 'background.paper',
                                                                padding: '8px',
                                                                borderRadius: '4px'
                                                            }}>
                                        {testResults[service.id].response}
                                    </pre>
                                                        </>
                                                    )}
                                                </Box>
                                            )}
                                        </Box>
                                    </Paper>
                                ))}
                            </List>

                            {apiServices.length === 0 && (
                                <Paper sx={{ p: 4, textAlign: 'center' }}>
                                    <Api sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
                                    <Typography variant="h6" sx={{ mb: 1 }}>{t('settings.apiServiceNoServices')}</Typography>
                                    <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                                        {t('settings.apiServiceNoServicesDescription')}
                                    </Typography>
                                    <Button variant="contained" startIcon={<Add />} onClick={handleAddApiService}>
                                        {t('settings.apiServiceAddFirst')}
                                    </Button>
                                </Paper>
                            )}
                        </TabPanel>

                        {/* OCR Settings */}
                        <TabPanel value={tabValue} index={3}>
                            <Grid container spacing={3}>
                                <Grid item xs={12} md={6}>
                                    <TextField
                                        fullWidth
                                        label={t('settings.screenshotQuality')}
                                        type="number"
                                        value={generalSettings.screenshot_quality}
                                        onChange={(e) => setGeneralSettings({ ...generalSettings, screenshot_quality: parseInt(e.target.value) })}
                                        InputProps={{ inputProps: { min: 1, max: 100 } }}
                                        helperText={t('settings.screenshotQualityHelp')}
                                    />
                                </Grid>
                                <Grid item xs={12} md={6}>
                                    <TextField
                                        fullWidth
                                        label={t('settings.ocrConfidenceThreshold')}
                                        type="number"
                                        value={generalSettings.ocr_confidence_threshold}
                                        onChange={(e) => setGeneralSettings({ ...generalSettings, ocr_confidence_threshold: parseInt(e.target.value) })}
                                        InputProps={{ inputProps: { min: 0, max: 100 } }}
                                        helperText={t('settings.ocrConfidenceThresholdHelp')}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <Button variant="contained" startIcon={<Save />} onClick={handleSaveGeneralSettings}>
                                        {t('settings.saveSettings')}
                                    </Button>
                                </Grid>
                            </Grid>
                        </TabPanel>

                        {/* Keywords */}
                        <TabPanel value={tabValue} index={4}>
                            <Box sx={{ mb: 3 }}>
                                <Typography variant="h6" sx={{ mb: 2 }}>{t('settings.spamDetectionKeywords')}</Typography>
                                <Alert severity="info" sx={{ mb: 2 }}>
                                    {t('settings.keywordsHelp')}
                                </Alert>
                                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                                    {keywords.map((keyword) => (
                                        <Chip
                                            key={keyword.id}
                                            label={keyword.keyword}
                                            onDelete={() => handleDeleteKeyword(keyword.id)}
                                            onClick={() => handleEditKeyword(keyword)}
                                            color={keyword.is_active ? 'primary' : 'default'}
                                            sx={{ m: 0.5 }}
                                        />
                                    ))}
                                    <Chip
                                        label={`+ ${t('settings.addKeyword')}`}
                                        onClick={handleAddKeyword}
                                        variant="outlined"
                                        sx={{ m: 0.5 }}
                                    />
                                </Box>
                            </Box>
                        </TabPanel>

                        {/* Schedules */}
                        <TabPanel value={tabValue} index={5}>
                            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Typography variant="h6">{t('settings.checkSchedules')}</Typography>
                                <Button variant="contained" startIcon={<Add />} onClick={handleAddSchedule}>
                                    {t('settings.addSchedule')}
                                </Button>
                            </Box>
                            <List>
                                {schedules.map((schedule) => (
                                    <ListItem key={schedule.id} sx={{ bgcolor: 'background.paper', mb: 1, borderRadius: 1 }}>
                                        <ListItemText
                                            primary={schedule.name}
                                            secondary={`${t('settings.expression')}: ${schedule.cron_expression}${schedule.last_run ? ` • ${t('settings.lastRun')}: ${new Date(schedule.last_run).toLocaleString()}` : ''}`}
                                        />
                                        <ListItemSecondaryAction>
                                            <FormControlLabel
                                                control={<Switch checked={schedule.is_active} onChange={(e) => handleToggleSchedule(schedule, e.target.checked)} />}
                                                label={t('common.active')}
                                            />
                                            <IconButton edge="end" onClick={() => handleEditSchedule(schedule)}>
                                                <Edit />
                                            </IconButton>
                                            <IconButton edge="end" color="error" onClick={() => handleDeleteSchedule(schedule.id)}>
                                                <Delete />
                                            </IconButton>
                                        </ListItemSecondaryAction>
                                    </ListItem>
                                ))}
                            </List>
                        </TabPanel>

                        {/* Notifications */}
                        <TabPanel value={tabValue} index={6}>
                            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <Typography variant="h6">{t('settings.notificationChannels')}</Typography>
                                <Button variant="contained" startIcon={<Add />} onClick={handleAddNotification}>
                                    {t('settings.addChannel')}
                                </Button>
                            </Box>
                            <List>
                                {notifications.map((notification) => {
                                    const config = getNotificationConfig(notification);
                                    return (
                                        <ListItem key={notification.id} sx={{ bgcolor: 'background.paper', mb: 1, borderRadius: 1 }}>
                                            <ListItemText
                                                primary={notification.type.charAt(0).toUpperCase() + notification.type.slice(1)}
                                                secondary={notification.type === 'telegram' ? `Chat: ${config.chat_id || 'Not configured'}` : `To: ${config.to_emails?.join(', ') || 'Not configured'}`}
                                            />
                                            <ListItemSecondaryAction>
                                                <Button size="small" onClick={() => handleTestNotification(notification.id)}>
                                                    {t('settings.testNotification')}
                                                </Button>
                                                <FormControlLabel
                                                    control={<Switch checked={notification.is_active} onChange={(e) => handleToggleNotification(notification, e.target.checked)} />}
                                                    label={t('common.active')}
                                                />
                                                <IconButton edge="end" onClick={() => handleEditNotification(notification)}>
                                                    <Edit />
                                                </IconButton>
                                                <IconButton edge="end" color="error" onClick={() => handleDeleteNotification(notification.id)}>
                                                    <Delete />
                                                </IconButton>
                                            </ListItemSecondaryAction>
                                        </ListItem>
                                    );
                                })}
                            </List>
                        </TabPanel>

                        {/* Database */}
                        <TabPanel value={tabValue} index={7}>
                            <Typography variant="h6" sx={{ mb: 3 }}>{t('settings.databaseConfiguration')}</Typography>
                            <Alert severity="info">
                                {t('settings.databaseConfigHelp')}
                            </Alert>
                        </TabPanel>
                    </CardContent>
                </Card>
            )}

            {/* API Service Dialog */}
            <Dialog open={apiDialogOpen} onClose={() => setApiDialogOpen(false)} maxWidth="md" fullWidth>
                <DialogTitle>
                    {editingApiService?.id === 0 ? t('settings.apiServiceDialogAddTitle') : t('settings.apiServiceDialogEditTitle')}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ mt: 2 }}>
                        <Grid container spacing={2}>
                            <Grid item xs={12}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogServiceName')}
                                    value={editingApiService?.name || ''}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, name: e.target.value } : null)}
                                    helperText={t('settings.apiServiceDialogServiceNameHelper')}
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogServiceCode')}
                                    value={editingApiService?.service_code || 'custom_api'}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, service_code: e.target.value } : null)}
                                    helperText={t('settings.apiServiceDialogServiceCodeHelper')}
                                />
                            </Grid>
                            <Grid item xs={12} md={3}>
                                <FormControl fullWidth>
                                    <InputLabel>{t('settings.apiServiceDialogMethod')}</InputLabel>
                                    <Select
                                        value={editingApiService?.method || 'GET'}
                                        label={t('settings.apiServiceDialogMethod')}
                                        onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, method: e.target.value } : null)}
                                    >
                                        <MenuItem value="GET">GET</MenuItem>
                                        <MenuItem value="POST">POST</MenuItem>
                                        <MenuItem value="PUT">PUT</MenuItem>
                                    </Select>
                                </FormControl>
                            </Grid>
                            <Grid item xs={12} md={3}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogTimeout')}
                                    type="number"
                                    value={editingApiService?.timeout || 30}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, timeout: parseInt(e.target.value) } : null)}
                                    InputProps={{ inputProps: { min: 1, max: 300 } }}
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogApiUrl')}
                                    value={editingApiService?.api_url || ''}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, api_url: e.target.value } : null)}
                                    helperText={t('settings.apiServiceDialogApiUrlHelper')}
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                                    <Typography variant="subtitle2">{t('settings.apiServiceDialogHeaders')}</Typography>
                                    <Button
                                        size="small"
                                        startIcon={<Code />}
                                        onClick={() => handleOpenHeadersEditor(editingApiService?.headers || '{}')}
                                    >
                                        {t('settings.apiServiceDialogEditHeaders')}
                                    </Button>
                                </Box>
                                <TextField
                                    fullWidth
                                    multiline
                                    rows={4}
                                    value={editingApiService?.headers || '{}'}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, headers: e.target.value } : null)}
                                    error={!validateJson(editingApiService?.headers || '{}')}
                                    helperText={!validateJson(editingApiService?.headers || '{}') ? t('settings.apiServiceDialogInvalidJson') : t('settings.apiServiceDialogHeadersHelper')}
                                    sx={{ fontFamily: 'monospace', fontSize: '12px' }}
                                />
                            </Grid>
                            {editingApiService?.method !== 'GET' && (
                                <Grid item xs={12}>
                                    <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceDialogRequestBody')}</Typography>
                                    <TextField
                                        fullWidth
                                        multiline
                                        rows={4}
                                        value={editingApiService?.request_body || '{}'}
                                        onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, request_body: e.target.value } : null)}
                                        error={!validateJson(editingApiService?.request_body || '{}')}
                                        helperText={!validateJson(editingApiService?.request_body || '{}') ? t('settings.apiServiceDialogInvalidJson') : t('settings.apiServiceDialogRequestBodyHelper')}
                                        sx={{ fontFamily: 'monospace', fontSize: '12px' }}
                                    />
                                </Grid>
                            )}
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogResponsePath')}
                                    value={editingApiService?.response_path || ''}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, response_path: e.target.value } : null)}
                                    helperText={t('settings.apiServiceDialogResponsePathHelper')}
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label={t('settings.apiServiceDialogKeywordPaths')}
                                    value={editingApiService?.keyword_paths || ''}
                                    onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, keyword_paths: e.target.value } : null)}
                                    helperText={t('settings.apiServiceDialogKeywordPathsHelper')}
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={editingApiService?.is_active || false}
                                            onChange={(e) => setEditingApiService(editingApiService ? { ...editingApiService, is_active: e.target.checked } : null)}
                                        />
                                    }
                                    label={t('settings.apiServiceDialogActive')}
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <Alert severity="info" icon={<InfoIcon />}>
                                    <Typography variant="subtitle2" sx={{ mb: 1 }}>{t('settings.apiServiceDialogExampleConfigs')}</Typography>
                                    <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                                        {Object.entries(exampleConfigs).map(([name, config]) => (
                                            <Button
                                                key={name}
                                                size="small"
                                                variant="outlined"
                                                startIcon={<ContentCopy />}
                                                onClick={() => {
                                                    if (editingApiService) {
                                                        setEditingApiService({
                                                            ...editingApiService,
                                                            headers: JSON.stringify(config.headers, null, 2),
                                                            response_path: config.response_path || '',
                                                            keyword_paths: config.keyword_paths || '',
                                                        });
                                                        enqueueSnackbar(t('settings.apiServiceDialogConfigLoaded', { name: name.charAt(0).toUpperCase() + name.slice(1) }), { variant: 'info' });
                                                    }
                                                }}
                                            >
                                                {name.charAt(0).toUpperCase() + name.slice(1)}
                                            </Button>
                                        ))}
                                    </Box>
                                </Alert>
                            </Grid>
                        </Grid>
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setApiDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveApiService} variant="contained">{t('common.save')}</Button>
                </DialogActions>
            </Dialog>

            {/* Headers Editor Dialog */}
            <Dialog open={headersEditorOpen} onClose={() => setHeadersEditorOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>Edit Headers</DialogTitle>
                <DialogContent>
                    <Box sx={{ mt: 2 }}>
                        <TextField
                            fullWidth
                            multiline
                            rows={15}
                            value={editingHeaders}
                            onChange={(e) => setEditingHeaders(e.target.value)}
                            error={!validateJson(editingHeaders)}
                            helperText={!validateJson(editingHeaders) ? 'Invalid JSON format' : 'Edit headers in JSON format'}
                            sx={{ fontFamily: 'monospace', fontSize: '12px' }}
                        />
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setHeadersEditorOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveHeaders} variant="contained" disabled={!validateJson(editingHeaders)}>
                        {t('common.save')}
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Gateway Dialog */}
            <Dialog open={adbDialogOpen} onClose={() => setAdbDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{editingGateway?.id === 0 ? t('settings.addGateway') : t('settings.editGateway')}</DialogTitle>
                <DialogContent>
                    <Box sx={{ mt: 2 }}>
                        {editingGateway?.id === 0 && (
                            <>
                                <Typography variant="subtitle2" sx={{ mb: 2 }}>Gateway Type</Typography>
                                <RadioGroup
                                    value={gatewayCreationType}
                                    onChange={(e) => setGatewayCreationType(e.target.value as 'manual' | 'docker')}
                                    sx={{ mb: 3 }}
                                >
                                    <FormControlLabel
                                        value="manual"
                                        control={<Radio />}
                                        label={
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                <Computer />
                                                <Box>
                                                    <Typography variant="body1">Manual Configuration</Typography>
                                                    <Typography variant="caption" color="text.secondary">
                                                        Connect to existing Android emulator
                                                    </Typography>
                                                </Box>
                                            </Box>
                                        }
                                    />
                                    <FormControlLabel
                                        value="docker"
                                        control={<Radio />}
                                        label={
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                <DockerIcon/>
                                                <Box>
                                                    <Typography variant="body1">Docker Container</Typography>
                                                    <Typography variant="caption" color="text.secondary">
                                                        Create new Android emulator in Docker
                                                    </Typography>
                                                </Box>
                                            </Box>
                                        }
                                    />
                                </RadioGroup>
                                <Divider sx={{ mb: 3 }} />
                            </>
                        )}

                        <Grid container spacing={2}>
                            <Grid item xs={12}>
                                <TextField
                                    fullWidth
                                    label={t('settings.gatewayName')}
                                    value={editingGateway?.name || ''}
                                    onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, name: e.target.value } : null)}
                                />
                            </Grid>

                            {(gatewayCreationType === 'manual' || editingGateway?.id !== 0) && !editingGateway?.is_docker && (
                                <>
                                    <Grid item xs={12} md={8}>
                                        <TextField
                                            fullWidth
                                            label={t('settings.host')}
                                            value={editingGateway?.host || ''}
                                            onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, host: e.target.value } : null)}
                                        />
                                    </Grid>
                                    <Grid item xs={12} md={4}>
                                        <TextField
                                            fullWidth
                                            label={t('settings.port')}
                                            type="number"
                                            value={editingGateway?.port || 5554}
                                            onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, port: parseInt(e.target.value) } : null)}
                                        />
                                    </Grid>
                                </>
                            )}

                            <Grid item xs={12}>
                                <FormControl fullWidth>
                                    <InputLabel>{t('settings.serviceCode')}</InputLabel>
                                    <Select
                                        value={editingGateway?.service_code || 'yandex_aon'}
                                        label={t('settings.serviceCode')}
                                        onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, service_code: e.target.value } : null)}
                                    >
                                        <MenuItem value="yandex_aon">Yandex АОН</MenuItem>
                                        <MenuItem value="kaspersky">Kaspersky Who Calls</MenuItem>
                                        <MenuItem value="getcontact">GetContact</MenuItem>
                                    </Select>
                                </FormControl>
                            </Grid>

                            {gatewayCreationType === 'docker' && editingGateway?.id === 0 && (
                                <Grid item xs={12}>
                                    <Alert severity="info" sx={{ mb: 2 }}>
                                        Docker container will be created with Android emulator. You can optionally upload an APK file to install automatically.
                                    </Alert>
                                    <Button
                                        variant="outlined"
                                        component="label"
                                        fullWidth
                                        startIcon={<CloudUpload />}
                                    >
                                        {dockerAPKFile ? dockerAPKFile.name : 'Upload APK (Optional)'}
                                        <input
                                            type="file"
                                            hidden
                                            accept=".apk"
                                            onChange={(e) => setDockerAPKFile(e.target.files?.[0] || null)}
                                        />
                                    </Button>
                                </Grid>
                            )}

                            {editingGateway?.id !== 0 && !editingGateway?.is_docker && (
                                <Grid item xs={12}>
                                    <FormControlLabel
                                        control={
                                            <Switch
                                                checked={editingGateway?.is_active || false}
                                                onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, is_active: e.target.checked } : null)}
                                            />
                                        }
                                        label={t('common.active')}
                                    />
                                </Grid>
                            )}
                        </Grid>
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setAdbDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveGateway} variant="contained">{t('common.save')}</Button>
                </DialogActions>
            </Dialog>

            {/* Keyword Dialog */}
            <Dialog open={keywordDialogOpen} onClose={() => setKeywordDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{editingKeyword?.id === 0 ? t('settings.addKeyword') : t('settings.keyword')}</DialogTitle>
                <DialogContent>
                    <TextField
                        fullWidth
                        label={t('settings.keyword')}
                        value={editingKeyword?.keyword || ''}
                        onChange={(e) => setEditingKeyword(editingKeyword ? { ...editingKeyword, keyword: e.target.value } : null)}
                        sx={{ mt: 2 }}
                    />
                    <FormControlLabel
                        control={
                            <Switch
                                checked={editingKeyword?.is_active || false}
                                onChange={(e) => setEditingKeyword(editingKeyword ? { ...editingKeyword, is_active: e.target.checked } : null)}
                            />
                        }
                        label={t('common.active')}
                        sx={{ mt: 2 }}
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setKeywordDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveKeyword} variant="contained">{t('common.save')}</Button>
                </DialogActions>
            </Dialog>

            {/* Schedule Dialog */}
            <Dialog open={scheduleDialogOpen} onClose={() => setScheduleDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{editingSchedule?.id === 0 ? t('settings.addSchedule') : t('settings.editSchedule')}</DialogTitle>
                <DialogContent>
                    <Grid container spacing={2} sx={{ mt: 1 }}>
                        <Grid item xs={12}>
                            <TextField
                                fullWidth
                                label={t('settings.scheduleName')}
                                value={editingSchedule?.name || ''}
                                onChange={(e) => setEditingSchedule(editingSchedule ? { ...editingSchedule, name: e.target.value } : null)}
                            />
                        </Grid>
                        <Grid item xs={12}>
                            <FormControl fullWidth>
                                <InputLabel>{t('settings.cronExpression')}</InputLabel>
                                <Select
                                    value={editingSchedule?.cron_expression || '@hourly'}
                                    label={t('settings.cronExpression')}
                                    onChange={(e) => setEditingSchedule(editingSchedule ? { ...editingSchedule, cron_expression: e.target.value } : null)}
                                >
                                    <MenuItem value="@hourly">Every hour</MenuItem>
                                    <MenuItem value="@daily">Every day</MenuItem>
                                    <MenuItem value="@weekly">Every week</MenuItem>
                                    <MenuItem value="0 */6 * * *">Every 6 hours</MenuItem>
                                    <MenuItem value="0 */12 * * *">Every 12 hours</MenuItem>
                                </Select>
                            </FormControl>
                        </Grid>
                        <Grid item xs={12}>
                            <FormControlLabel
                                control={
                                    <Switch
                                        checked={editingSchedule?.is_active || false}
                                        onChange={(e) => setEditingSchedule(editingSchedule ? { ...editingSchedule, is_active: e.target.checked } : null)}
                                    />
                                }
                                label={t('common.active')}
                            />
                        </Grid>
                    </Grid>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setScheduleDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveSchedule} variant="contained">{t('common.save')}</Button>
                </DialogActions>
            </Dialog>

            {/* Notification Dialog */}
            <Dialog open={notificationDialogOpen} onClose={() => setNotificationDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{editingNotification?.id === 0 ? t('settings.addChannel') : t('settings.editChannel')}</DialogTitle>
                <DialogContent>
                    <Grid container spacing={2} sx={{ mt: 1 }}>
                        <Grid item xs={12}>
                            <FormControl fullWidth>
                                <InputLabel>{t('settings.channelType')}</InputLabel>
                                <Select
                                    value={editingNotification?.type || 'telegram'}
                                    label={t('settings.channelType')}
                                    onChange={(e) => setEditingNotification(editingNotification ? { ...editingNotification, type: e.target.value } : null)}
                                    disabled={editingNotification?.id !== 0}
                                >
                                    <MenuItem value="telegram">Telegram</MenuItem>
                                    <MenuItem value="email">Email</MenuItem>
                                </Select>
                            </FormControl>
                        </Grid>
                        {editingNotification?.type === 'telegram' ? (
                            <>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="Bot Token"
                                        value={editingNotification?.config?.bot_token || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, bot_token: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="Chat ID"
                                        value={editingNotification?.config?.chat_id || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, chat_id: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                            </>
                        ) : (
                            <>
                                <Grid item xs={12} md={8}>
                                    <TextField
                                        fullWidth
                                        label="SMTP Host"
                                        value={editingNotification?.config?.smtp_host || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, smtp_host: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12} md={4}>
                                    <TextField
                                        fullWidth
                                        label="SMTP Port"
                                        value={editingNotification?.config?.smtp_port || '587'}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, smtp_port: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="SMTP User"
                                        value={editingNotification?.config?.smtp_user || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, smtp_user: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="SMTP Password"
                                        type="password"
                                        value={editingNotification?.config?.smtp_password || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, smtp_password: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="From Email"
                                        value={editingNotification?.config?.from_email || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, from_email: e.target.value }
                                        } : null)}
                                    />
                                </Grid>
                                <Grid item xs={12}>
                                    <TextField
                                        fullWidth
                                        label="To Emails (comma separated)"
                                        value={editingNotification?.config?.to_emails?.join(', ') || ''}
                                        onChange={(e) => setEditingNotification(editingNotification ? {
                                            ...editingNotification,
                                            config: { ...editingNotification.config, to_emails: e.target.value.split(',').map(email => email.trim()) }
                                        } : null)}
                                    />
                                </Grid>
                            </>
                        )}
                        <Grid item xs={12}>
                            <FormControlLabel
                                control={
                                    <Switch
                                        checked={editingNotification?.is_active || false}
                                        onChange={(e) => setEditingNotification(editingNotification ? { ...editingNotification, is_active: e.target.checked } : null)}
                                    />
                                }
                                label={t('common.active')}
                            />
                        </Grid>
                    </Grid>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setNotificationDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSaveNotification} variant="contained">{t('common.save')}</Button>
                </DialogActions>
            </Dialog>
        </Box>
    );
});

export default SettingsPage;