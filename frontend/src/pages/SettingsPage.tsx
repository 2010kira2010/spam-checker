import React, { useState, useEffect } from 'react';
import { observer } from 'mobx-react-lite';
import { useTranslation } from 'react-i18next';
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
    PlayArrow,
    WifiTethering,
    Refresh,
    Language,
} from '@mui/icons-material';
import { useSnackbar } from 'notistack';
import axios from 'axios';

interface GeneralSettings {
    check_interval_minutes: number;
    max_concurrent_checks: number;
    notification_batch_size: number;
    screenshot_quality: number;
    ocr_confidence_threshold: number;
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
    });

    // ADB Gateways
    const [adbGateways, setAdbGateways] = useState<ADBGateway[]>([]);
    const [adbDialogOpen, setAdbDialogOpen] = useState(false);
    const [editingGateway, setEditingGateway] = useState<ADBGateway | null>(null);

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
            const [settingsRes, gatewaysRes, keywordsRes, schedulesRes, notificationsRes] = await Promise.all([
                axios.get('/settings'),
                axios.get('/adb/gateways'),
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
            setGeneralSettings(settings as GeneralSettings);

            setAdbGateways(gatewaysRes.data);
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
        });
        setAdbDialogOpen(true);
    };

    const handleEditGateway = (gateway: ADBGateway) => {
        setEditingGateway(gateway);
        setAdbDialogOpen(true);
    };

    const handleSaveGateway = async () => {
        if (!editingGateway) return;

        try {
            if (editingGateway.id === 0) {
                const res = await axios.post('/adb/gateways', editingGateway);
                setAdbGateways([...adbGateways, res.data]);
            } else {
                await axios.put(`/adb/gateways/${editingGateway.id}`, editingGateway);
                setAdbGateways(adbGateways.map(g => g.id === editingGateway.id ? editingGateway : g));
            }
            setAdbDialogOpen(false);
            enqueueSnackbar(t('common.success'), { variant: 'success' });
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

    return (
        <Box>
            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="h4" sx={{ fontWeight: 600 }}>
                    {t('settings.title')}
                </Typography>
                <FormControl size="small">
                    <Select
                        value={i18n.language}
                        onChange={(e) => changeLanguage(e.target.value)}
                        startAdornment={<Language sx={{ mr: 1, color: 'text.secondary' }} />}
                    >
                        <MenuItem value="en">English</MenuItem>
                        <MenuItem value="ru">Русский</MenuItem>
                    </Select>
                </FormControl>
            </Box>

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
                                                    <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                                                        {gateway.name}
                                                    </Typography>
                                                    <Typography variant="body2" color="text.secondary">
                                                        {gateway.host}:{gateway.port} • {t('settings.serviceCode')}: {gateway.service_code}
                                                    </Typography>
                                                </Box>
                                                <Chip
                                                    label={t(`settings.${gateway.status}`)}
                                                    size="small"
                                                    color={getStatusColor(gateway.status)}
                                                />
                                            </Box>
                                            <Box>
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

                        {/* OCR Settings */}
                        <TabPanel value={tabValue} index={2}>
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
                        <TabPanel value={tabValue} index={3}>
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
                        <TabPanel value={tabValue} index={4}>
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
                        <TabPanel value={tabValue} index={5}>
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
                        <TabPanel value={tabValue} index={6}>
                            <Typography variant="h6" sx={{ mb: 3 }}>{t('settings.databaseConfiguration')}</Typography>
                            <Alert severity="info">
                                {t('settings.databaseConfigHelp')}
                            </Alert>
                        </TabPanel>
                    </CardContent>
                </Card>
            )}

            {/* Gateway Dialog */}
            <Dialog open={adbDialogOpen} onClose={() => setAdbDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{editingGateway?.id === 0 ? t('settings.addGateway') : t('settings.editGateway')}</DialogTitle>
                <DialogContent>
                    <Grid container spacing={2} sx={{ mt: 1 }}>
                        <Grid item xs={12}>
                            <TextField
                                fullWidth
                                label={t('settings.gatewayName')}
                                value={editingGateway?.name || ''}
                                onChange={(e) => setEditingGateway(editingGateway ? { ...editingGateway, name: e.target.value } : null)}
                            />
                        </Grid>
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
                    </Grid>
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