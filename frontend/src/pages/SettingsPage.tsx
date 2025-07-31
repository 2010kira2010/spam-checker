import React, { useState } from 'react';
import { observer } from 'mobx-react-lite';
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
} from '@mui/icons-material';
import { useSnackbar } from 'notistack';

interface TelegramNotification {
    id: number;
    type: 'telegram';
    is_active: boolean;
    config: {
        chat_id: string;
    };
}

interface EmailNotification {
    id: number;
    type: 'email';
    is_active: boolean;
    config: {
        to_emails: string[];
    };
}

type Notification = TelegramNotification | EmailNotification;

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
    const { enqueueSnackbar } = useSnackbar();
    const [tabValue, setTabValue] = useState(0);
    const [adbDialogOpen, setAdbDialogOpen] = useState(false);
    const [notificationDialogOpen, setNotificationDialogOpen] = useState(false);

    const getNotificationSecondary = (notification: Notification): string => {
        if (notification.type === 'telegram') {
            return `Chat: ${notification.config.chat_id}`;
        } else {
            return `To: ${notification.config.to_emails.join(', ')}`;
        }
    };
    const [scheduleDialogOpen, setScheduleDialogOpen] = useState(false);

    // Mock data
    const [settings, setSettings] = useState({
        check_interval_minutes: 60,
        max_concurrent_checks: 3,
        screenshot_quality: 80,
        ocr_confidence_threshold: 70,
        notification_batch_size: 50,
        tesseract_path: '/usr/bin/tesseract',
        ocr_language: 'rus+eng',
    });

    const [adbGateways] = useState([
        { id: 1, name: 'Yandex АОН', host: 'android-yandex', port: 5554, status: 'online', service_code: 'yandex_aon' },
        { id: 2, name: 'Kaspersky', host: 'android-kaspersky', port: 5554, status: 'offline', service_code: 'kaspersky' },
        { id: 3, name: 'GetContact', host: 'android-getcontact', port: 5554, status: 'online', service_code: 'getcontact' },
    ]);

    const [notifications, setNotifications] = useState<Notification[]>([
        {
            id: 1,
            type: 'telegram',
            is_active: true,
            config: { chat_id: '@spamchecker' }
        },
        {
            id: 2,
            type: 'email',
            is_active: false,
            config: { to_emails: ['admin@company.com'] }
        },
    ]);

    const [schedules] = useState([
        { id: 1, name: 'Hourly Check', cron_expression: '@hourly', is_active: true },
        { id: 2, name: 'Daily Morning', cron_expression: '0 9 * * *', is_active: true },
        { id: 3, name: 'Weekly Report', cron_expression: '@weekly', is_active: false },
    ]);

    const [keywords] = useState([
        'спам', 'реклама', 'мошенник', 'развод', 'коллектор',
        'банк', 'кредит', 'микрозайм', 'spam', 'scam', 'fraud'
    ]);

    const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
        setTabValue(newValue);
    };

    const handleSaveSettings = () => {
        enqueueSnackbar('Settings saved successfully', { variant: 'success' });
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

    return (
        <Box>
            <Typography variant="h4" sx={{ mb: 3, fontWeight: 600 }}>
                Settings
            </Typography>

            <Card>
                <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                    <Tabs value={tabValue} onChange={handleTabChange} variant="scrollable" scrollButtons="auto">
                        <Tab icon={<SettingsIcon />} label="General" />
                        <Tab icon={<Android />} label="ADB Gateways" />
                        <Tab icon={<Scanner />} label="OCR Settings" />
                        <Tab icon={<TextFields />} label="Keywords" />
                        <Tab icon={<Schedule />} label="Schedules" />
                        <Tab icon={<Notifications />} label="Notifications" />
                        <Tab icon={<Storage />} label="Database" />
                    </Tabs>
                </Box>

                <CardContent>
                    {/* General Settings */}
                    <TabPanel value={tabValue} index={0}>
                        <Grid container spacing={3}>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Check Interval (minutes)"
                                    type="number"
                                    value={settings.check_interval_minutes}
                                    onChange={(e) => setSettings({ ...settings, check_interval_minutes: parseInt(e.target.value) })}
                                    helperText="How often to check all active phones"
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Max Concurrent Checks"
                                    type="number"
                                    value={settings.max_concurrent_checks}
                                    onChange={(e) => setSettings({ ...settings, max_concurrent_checks: parseInt(e.target.value) })}
                                    helperText="Maximum number of phones to check simultaneously"
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Notification Batch Size"
                                    type="number"
                                    value={settings.notification_batch_size}
                                    onChange={(e) => setSettings({ ...settings, notification_batch_size: parseInt(e.target.value) })}
                                    helperText="Number of results to include in one notification"
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <Button variant="contained" startIcon={<Save />} onClick={handleSaveSettings}>
                                    Save Settings
                                </Button>
                            </Grid>
                        </Grid>
                    </TabPanel>

                    {/* ADB Gateways */}
                    <TabPanel value={tabValue} index={1}>
                        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <Typography variant="h6">Android Debug Bridge Gateways</Typography>
                            <Button variant="contained" startIcon={<Add />} onClick={() => setAdbDialogOpen(true)}>
                                Add Gateway
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
                                                    {gateway.host}:{gateway.port} • Service: {gateway.service_code}
                                                </Typography>
                                            </Box>
                                            <Chip
                                                label={gateway.status}
                                                size="small"
                                                color={getStatusColor(gateway.status)}
                                            />
                                        </Box>
                                        <Box>
                                            <IconButton size="small">
                                                <PlayArrow />
                                            </IconButton>
                                            <IconButton size="small">
                                                <Edit />
                                            </IconButton>
                                            <IconButton size="small" color="error">
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
                            <Grid item xs={12}>
                                <TextField
                                    fullWidth
                                    label="Tesseract Path"
                                    value={settings.tesseract_path}
                                    onChange={(e) => setSettings({ ...settings, tesseract_path: e.target.value })}
                                    helperText="Path to Tesseract OCR executable"
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="OCR Language"
                                    value={settings.ocr_language}
                                    onChange={(e) => setSettings({ ...settings, ocr_language: e.target.value })}
                                    helperText="Language codes separated by + (e.g., rus+eng)"
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Screenshot Quality"
                                    type="number"
                                    value={settings.screenshot_quality}
                                    onChange={(e) => setSettings({ ...settings, screenshot_quality: parseInt(e.target.value) })}
                                    InputProps={{ inputProps: { min: 1, max: 100 } }}
                                    helperText="JPEG quality (1-100)"
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="OCR Confidence Threshold"
                                    type="number"
                                    value={settings.ocr_confidence_threshold}
                                    onChange={(e) => setSettings({ ...settings, ocr_confidence_threshold: parseInt(e.target.value) })}
                                    InputProps={{ inputProps: { min: 0, max: 100 } }}
                                    helperText="Minimum confidence level for OCR results (0-100)"
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <Button variant="contained" startIcon={<Save />} onClick={handleSaveSettings}>
                                    Save OCR Settings
                                </Button>
                            </Grid>
                        </Grid>
                    </TabPanel>

                    {/* Keywords */}
                    <TabPanel value={tabValue} index={3}>
                        <Box sx={{ mb: 3 }}>
                            <Typography variant="h6" sx={{ mb: 2 }}>Spam Detection Keywords</Typography>
                            <Alert severity="info" sx={{ mb: 2 }}>
                                These keywords will be searched in OCR results to determine if a number is spam
                            </Alert>
                            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                                {keywords.map((keyword, index) => (
                                    <Chip
                                        key={index}
                                        label={keyword}
                                        onDelete={() => {}}
                                        sx={{ m: 0.5 }}
                                    />
                                ))}
                                <Chip
                                    label="+ Add Keyword"
                                    onClick={() => {}}
                                    variant="outlined"
                                    sx={{ m: 0.5 }}
                                />
                            </Box>
                        </Box>
                    </TabPanel>

                    {/* Schedules */}
                    <TabPanel value={tabValue} index={4}>
                        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <Typography variant="h6">Check Schedules</Typography>
                            <Button variant="contained" startIcon={<Add />} onClick={() => setScheduleDialogOpen(true)}>
                                Add Schedule
                            </Button>
                        </Box>
                        <List>
                            {schedules.map((schedule) => (
                                <ListItem key={schedule.id} sx={{ bgcolor: 'background.paper', mb: 1, borderRadius: 1 }}>
                                    <ListItemText
                                        primary={schedule.name}
                                        secondary={`Expression: ${schedule.cron_expression}`}
                                    />
                                    <ListItemSecondaryAction>
                                        <FormControlLabel
                                            control={<Switch checked={schedule.is_active} />}
                                            label="Active"
                                        />
                                        <IconButton edge="end">
                                            <Edit />
                                        </IconButton>
                                        <IconButton edge="end" color="error">
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
                            <Typography variant="h6">Notification Channels</Typography>
                            <Button variant="contained" startIcon={<Add />} onClick={() => setNotificationDialogOpen(true)}>
                                Add Channel
                            </Button>
                        </Box>
                        <List>
                            {notifications.map((notification) => (
                                <ListItem key={notification.id} sx={{ bgcolor: 'background.paper', mb: 1, borderRadius: 1 }}>
                                    <ListItemText
                                        primary={notification.type.charAt(0).toUpperCase() + notification.type.slice(1)}
                                        secondary={notification.type === 'telegram' ? `Chat: ${notification.config.chat_id}` : `To: ${notification.config.to_emails.join(', ')}`}
                                    />
                                    <ListItemSecondaryAction>
                                        <FormControlLabel
                                            control={<Switch checked={notification.is_active} />}
                                            label="Active"
                                        />
                                        <IconButton edge="end">
                                            <Edit />
                                        </IconButton>
                                        <IconButton edge="end" color="error">
                                            <Delete />
                                        </IconButton>
                                    </ListItemSecondaryAction>
                                </ListItem>
                            ))}
                        </List>
                    </TabPanel>

                    {/* Database */}
                    <TabPanel value={tabValue} index={6}>
                        <Typography variant="h6" sx={{ mb: 3 }}>Database Configuration</Typography>
                        <Grid container spacing={3}>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Host"
                                    value="postgres"
                                    disabled
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Port"
                                    value="5432"
                                    disabled
                                />
                            </Grid>
                            <Grid item xs={12} md={6}>
                                <TextField
                                    fullWidth
                                    label="Database Name"
                                    value="spamchecker"
                                    disabled
                                />
                            </Grid>
                            <Grid item xs={12}>
                                <Alert severity="info">
                                    Database connection settings are configured through environment variables
                                </Alert>
                            </Grid>
                        </Grid>
                    </TabPanel>
                </CardContent>
            </Card>
        </Box>
    );
});

export default SettingsPage;