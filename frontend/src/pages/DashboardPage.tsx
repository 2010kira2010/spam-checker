import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
import { useTranslation } from 'react-i18next';
import {
    Box,
    Grid,
    Card,
    CardContent,
    Typography,
    Paper,
    LinearProgress,
    Chip,
    IconButton,
    Tooltip,
    useTheme,
    alpha,
} from '@mui/material';
import {
    Phone,
    CheckCircle,
    Warning,
    TrendingUp,
    TrendingDown,
    Refresh,
    AccessTime,
    PhoneInTalk,
} from '@mui/icons-material';
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip as RechartsTooltip,
    ResponsiveContainer,
    PieChart,
    Pie,
    Cell,
} from 'recharts';
import { format } from 'date-fns';
import { phoneStore } from '../stores/PhoneStore';
import { useSnackbar } from 'notistack';
import axios from 'axios';

interface StatCard {
    title: string;
    value: number | string;
    icon: React.ReactNode;
    trend?: number;
    color: string;
    subtitle?: string;
}

interface TimeSeriesData {
    date: string;
    total_checks: number;
    spam_count: number;
    clean_count: number;
    spam_rate: number;
}

interface ServiceStat {
    service_id: number;
    service_name: string;
    service_code: string;
    total_checks: number;
    spam_count: number;
    spam_rate: number;
}

interface RecentDetection {
    phone_number: string;
    description: string;
    checked_at: string;
    service_name: string;
    found_keywords: string[] | null;
}

const DashboardPage: React.FC = observer(() => {
    const { t } = useTranslation();
    const theme = useTheme();
    const { enqueueSnackbar } = useSnackbar();
    const [isLoading, setIsLoading] = useState(true);
    const [lastCheckTime, setLastCheckTime] = useState<Date | null>(null);

    // Statistics data
    const [overviewStats, setOverviewStats] = useState({
        total_phones: 0,
        active_phones: 0,
        total_checks: 0,
        spam_detections: 0,
        spam_rate: 0,
        active_gateways: 0,
    });
    const [phoneStats, setPhoneStats] = useState({
        total_phones: 0,
        active_phones: 0,
        spam_phones: 0,
        clean_phones: 0,
    });
    const [timeSeriesData, setTimeSeriesData] = useState<TimeSeriesData[]>([]);
    const [serviceStats, setServiceStats] = useState<ServiceStat[]>([]);
    const [recentSpamDetections, setRecentSpamDetections] = useState<RecentDetection[]>([]);

    useEffect(() => {
        loadDashboardData();
    }, []);

    const loadDashboardData = async () => {
        setIsLoading(true);
        try {
            // Load multiple data sources in parallel
            const [statsResponse, phoneStatsResponse, timeSeriesResponse, serviceStatsResponse, recentSpamResponse] = await Promise.all([
                axios.get('/statistics/overview'),
                axios.get('/phones/stats'),
                axios.get('/statistics/timeseries?days=7'),
                axios.get('/statistics/services'),
                axios.get('/statistics/recent-spam?limit=5'),
            ]);

            setOverviewStats(statsResponse.data);
            setPhoneStats(phoneStatsResponse.data);
            setTimeSeriesData(timeSeriesResponse.data);
            setServiceStats(serviceStatsResponse.data);
            setRecentSpamDetections(recentSpamResponse.data);

            // Also fetch phone store stats
            await phoneStore.fetchStats();

            setLastCheckTime(new Date());
        } catch (error) {
            enqueueSnackbar(t('errors.loadFailed'), { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const statCards: StatCard[] = [
        {
            title: t('dashboard.totalPhones'),
            value: phoneStats.total_phones,
            icon: <Phone />,
            color: theme.palette.primary.main,
            subtitle: t('dashboard.registeredNumbers'),
        },
        {
            title: t('dashboard.activePhones'),
            value: phoneStats.active_phones,
            icon: <PhoneInTalk />,
            color: theme.palette.success.main,
            trend: calculateTrend(phoneStats.active_phones, phoneStats.total_phones),
            subtitle: t('dashboard.beingMonitored'),
        },
        {
            title: t('dashboard.spamDetected'),
            value: phoneStats.spam_phones,
            icon: <Warning />,
            color: theme.palette.error.main,
            trend: phoneStats.total_phones > 0 ? -Math.round((phoneStats.spam_phones / phoneStats.total_phones) * 100) : 0,
            subtitle: t('dashboard.markedAsSpam'),
        },
        {
            title: t('dashboard.cleanNumbers'),
            value: phoneStats.clean_phones,
            icon: <CheckCircle />,
            color: theme.palette.info.main,
            subtitle: t('dashboard.noSpamDetected'),
        },
    ];

    // Transform time series data for the chart
    const chartData = timeSeriesData.map(item => ({
        day: format(new Date(item.date), 'EEE'),
        spam: item.spam_count,
        clean: item.clean_count,
    }));

    // Transform service stats for pie chart
    const serviceDistribution = serviceStats.map(service => ({
        name: service.service_name,
        value: service.total_checks,
        color: getServiceColor(service.service_code),
    }));

    // Transform recent spam detections for activity list
    const recentActivity = recentSpamDetections.map(detection => ({
        time: format(new Date(detection.checked_at), 'HH:mm'),
        phone: detection.phone_number,
        service: detection.service_name,
        status: detection.found_keywords && detection.found_keywords.length > 0 ? 'spam' : 'clean',
    }));

    function calculateTrend(current: number, total: number): number {
        if (total === 0) return 0;
        return Math.round((current / total) * 100);
    }

    function getServiceColor(serviceCode: string): string {
        switch (serviceCode) {
            case 'yandex_aon':
                return '#FF6B6B';
            case 'kaspersky':
                return '#4ECDC4';
            case 'getcontact':
                return '#45B7D1';
            default:
                return '#95A5A6';
        }
    }

    return (
        <Box>
            {/* Header */}
            <Box sx={{ mb: 4 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                    <Typography variant="h4" sx={{ fontWeight: 600 }}>
                        {t('dashboard.title')}
                    </Typography>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                        {lastCheckTime && (
                            <Chip
                                icon={<AccessTime />}
                                label={`${t('dashboard.lastUpdate')}: ${format(lastCheckTime, 'HH:mm')}`}
                                size="small"
                                variant="outlined"
                            />
                        )}
                        <Tooltip title={t('common.refresh')}>
                            <IconButton onClick={loadDashboardData} disabled={isLoading}>
                                <Refresh />
                            </IconButton>
                        </Tooltip>
                    </Box>
                </Box>
                {isLoading && <LinearProgress />}
            </Box>

            {/* Stats Cards */}
            <Grid container spacing={3} sx={{ mb: 4 }}>
                {statCards.map((stat, index) => (
                    <Grid item xs={12} sm={6} md={3} key={index}>
                        <Card
                            sx={{
                                background: `linear-gradient(135deg, ${alpha(stat.color, 0.1)} 0%, ${alpha(
                                    stat.color,
                                    0.05
                                )} 100%)`,
                                border: `1px solid ${alpha(stat.color, 0.2)}`,
                                transition: 'transform 0.2s',
                                '&:hover': {
                                    transform: 'translateY(-4px)',
                                },
                            }}
                        >
                            <CardContent>
                                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                                    <Box
                                        sx={{
                                            width: 48,
                                            height: 48,
                                            borderRadius: 2,
                                            display: 'flex',
                                            alignItems: 'center',
                                            justifyContent: 'center',
                                            bgcolor: alpha(stat.color, 0.2),
                                            color: stat.color,
                                        }}
                                    >
                                        {stat.icon}
                                    </Box>
                                    {stat.trend !== undefined && stat.trend !== 0 && (
                                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                            {stat.trend > 0 ? (
                                                <TrendingUp sx={{ color: 'success.main', fontSize: 20 }} />
                                            ) : (
                                                <TrendingDown sx={{ color: 'error.main', fontSize: 20 }} />
                                            )}
                                            <Typography
                                                variant="body2"
                                                sx={{ color: stat.trend > 0 ? 'success.main' : 'error.main', ml: 0.5 }}
                                            >
                                                {Math.abs(stat.trend)}%
                                            </Typography>
                                        </Box>
                                    )}
                                </Box>
                                <Typography variant="h3" sx={{ fontWeight: 700, mb: 0.5 }}>
                                    {stat.value}
                                </Typography>
                                <Typography variant="subtitle1" sx={{ fontWeight: 500, mb: 0.5 }}>
                                    {stat.title}
                                </Typography>
                                {stat.subtitle && (
                                    <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                                        {stat.subtitle}
                                    </Typography>
                                )}
                            </CardContent>
                        </Card>
                    </Grid>
                ))}
            </Grid>

            {/* Charts */}
            <Grid container spacing={3} sx={{ mb: 4 }}>
                {/* Weekly Trend */}
                <Grid item xs={12} md={8}>
                    <Paper sx={{ p: 3, height: 400 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            {t('dashboard.weeklyTrend')}
                        </Typography>
                        {chartData.length > 0 ? (
                            <ResponsiveContainer width="100%" height="85%">
                                <AreaChart data={chartData}>
                                    <defs>
                                        <linearGradient id="colorSpam" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor={theme.palette.error.main} stopOpacity={0.8} />
                                            <stop offset="95%" stopColor={theme.palette.error.main} stopOpacity={0} />
                                        </linearGradient>
                                        <linearGradient id="colorClean" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor={theme.palette.success.main} stopOpacity={0.8} />
                                            <stop offset="95%" stopColor={theme.palette.success.main} stopOpacity={0} />
                                        </linearGradient>
                                    </defs>
                                    <CartesianGrid strokeDasharray="3 3" stroke={alpha(theme.palette.divider, 0.3)} />
                                    <XAxis dataKey="day" stroke={theme.palette.text.secondary} />
                                    <YAxis stroke={theme.palette.text.secondary} />
                                    <RechartsTooltip
                                        contentStyle={{
                                            backgroundColor: theme.palette.background.paper,
                                            border: `1px solid ${theme.palette.divider}`,
                                            borderRadius: 8,
                                        }}
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="spam"
                                        stackId="1"
                                        stroke={theme.palette.error.main}
                                        fillOpacity={1}
                                        fill="url(#colorSpam)"
                                        name={t('phones.spam')}
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="clean"
                                        stackId="1"
                                        stroke={theme.palette.success.main}
                                        fillOpacity={1}
                                        fill="url(#colorClean)"
                                        name={t('phones.clean')}
                                    />
                                </AreaChart>
                            </ResponsiveContainer>
                        ) : (
                            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '85%' }}>
                                <Typography variant="body1" color="text.secondary">
                                    {t('common.noData', 'No data available')}
                                </Typography>
                            </Box>
                        )}
                    </Paper>
                </Grid>

                {/* Service Distribution */}
                <Grid item xs={12} md={4}>
                    <Paper sx={{ p: 3, height: 400 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            {t('dashboard.serviceDistribution')}
                        </Typography>
                        {serviceDistribution.length > 0 && serviceDistribution.some(s => s.value > 0) ? (
                            <>
                                <ResponsiveContainer width="100%" height="70%">
                                    <PieChart>
                                        <Pie
                                            data={serviceDistribution}
                                            cx="50%"
                                            cy="50%"
                                            innerRadius={60}
                                            outerRadius={80}
                                            paddingAngle={5}
                                            dataKey="value"
                                        >
                                            {serviceDistribution.map((entry, index) => (
                                                <Cell key={`cell-${index}`} fill={entry.color} />
                                            ))}
                                        </Pie>
                                        <RechartsTooltip />
                                    </PieChart>
                                </ResponsiveContainer>
                                <Box sx={{ display: 'flex', justifyContent: 'center', gap: 2, mt: 2, flexWrap: 'wrap' }}>
                                    {serviceDistribution.map((service, index) => (
                                        <Box key={index} sx={{ display: 'flex', alignItems: 'center' }}>
                                            <Box
                                                sx={{
                                                    width: 12,
                                                    height: 12,
                                                    borderRadius: '50%',
                                                    bgcolor: service.color,
                                                    mr: 1,
                                                }}
                                            />
                                            <Typography variant="caption">{service.name}</Typography>
                                        </Box>
                                    ))}
                                </Box>
                            </>
                        ) : (
                            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '85%' }}>
                                <Typography variant="body1" color="text.secondary">
                                    {t('common.noData', 'No data available')}
                                </Typography>
                            </Box>
                        )}
                    </Paper>
                </Grid>
            </Grid>

            {/* Recent Activity */}
            <Paper sx={{ p: 3 }}>
                <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                    {t('dashboard.recentActivity')}
                </Typography>
                {recentActivity.length > 0 ? (
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                        {recentActivity.map((activity, index) => (
                            <Box
                                key={index}
                                sx={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'space-between',
                                    p: 2,
                                    borderRadius: 2,
                                    bgcolor: alpha(theme.palette.background.default, 0.5),
                                    border: `1px solid ${alpha(theme.palette.divider, 0.1)}`,
                                }}
                            >
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                    <Typography variant="caption" sx={{ color: 'text.secondary', minWidth: 50 }}>
                                        {activity.time}
                                    </Typography>
                                    <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                        {activity.phone}
                                    </Typography>
                                    <Chip label={activity.service} size="small" variant="outlined" />
                                </Box>
                                <Chip
                                    label={t(`phones.${activity.status}`)}
                                    size="small"
                                    color={activity.status === 'spam' ? 'error' : 'success'}
                                    icon={activity.status === 'spam' ? <Warning /> : <CheckCircle />}
                                />
                            </Box>
                        ))}
                    </Box>
                ) : (
                    <Box sx={{ textAlign: 'center', py: 4 }}>
                        <Typography variant="body1" color="text.secondary">
                            {t('common.noActivity', 'No recent activity')}
                        </Typography>
                    </Box>
                )}
            </Paper>
        </Box>
    );
});

export default DashboardPage;