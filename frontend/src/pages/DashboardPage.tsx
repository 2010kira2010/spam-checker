import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
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

interface StatCard {
    title: string;
    value: number | string;
    icon: React.ReactNode;
    trend?: number;
    color: string;
    subtitle?: string;
}

const DashboardPage: React.FC = observer(() => {
    const theme = useTheme();
    const { enqueueSnackbar } = useSnackbar();
    const [isLoading, setIsLoading] = useState(true);
    const [lastCheckTime, setLastCheckTime] = useState<Date | null>(null);

    useEffect(() => {
        loadDashboardData();
    }, []);

    const loadDashboardData = async () => {
        setIsLoading(true);
        try {
            await phoneStore.fetchStats();
            await phoneStore.fetchPhones();
            setLastCheckTime(new Date());
        } catch (error) {
            enqueueSnackbar('Failed to load dashboard data', { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const statCards: StatCard[] = [
        {
            title: 'Total Phones',
            value: phoneStore.stats?.total_phones || 0,
            icon: <Phone />,
            color: theme.palette.primary.main,
            subtitle: 'Registered numbers',
        },
        {
            title: 'Active Phones',
            value: phoneStore.stats?.active_phones || 0,
            icon: <PhoneInTalk />,
            color: theme.palette.success.main,
            trend: 12,
            subtitle: 'Being monitored',
        },
        {
            title: 'Spam Detected',
            value: phoneStore.stats?.spam_phones || 0,
            icon: <Warning />,
            color: theme.palette.error.main,
            trend: -5,
            subtitle: 'Marked as spam',
        },
        {
            title: 'Clean Numbers',
            value: phoneStore.stats?.clean_phones || 0,
            icon: <CheckCircle />,
            color: theme.palette.info.main,
            subtitle: 'No spam detected',
        },
    ];

    // Mock data for charts
    const weeklyTrend = [
        { day: 'Mon', spam: 5, clean: 45 },
        { day: 'Tue', spam: 8, clean: 42 },
        { day: 'Wed', spam: 6, clean: 44 },
        { day: 'Thu', spam: 10, clean: 40 },
        { day: 'Fri', spam: 7, clean: 43 },
        { day: 'Sat', spam: 4, clean: 46 },
        { day: 'Sun', spam: 3, clean: 47 },
    ];

    const serviceDistribution = [
        { name: 'Yandex АОН', value: 25, color: '#FF6B6B' },
        { name: 'Kaspersky', value: 35, color: '#4ECDC4' },
        { name: 'GetContact', value: 40, color: '#45B7D1' },
    ];

    const recentActivity = [
        { time: '10:30', phone: '+7 (999) 123-45-67', service: 'Yandex АОН', status: 'spam' },
        { time: '10:25', phone: '+7 (999) 234-56-78', service: 'GetContact', status: 'clean' },
        { time: '10:20', phone: '+7 (999) 345-67-89', service: 'Kaspersky', status: 'spam' },
        { time: '10:15', phone: '+7 (999) 456-78-90', service: 'Yandex АОН', status: 'clean' },
    ];

    return (
        <Box>
            {/* Header */}
            <Box sx={{ mb: 4 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                    <Typography variant="h4" sx={{ fontWeight: 600 }}>
                        Dashboard
                    </Typography>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                        {lastCheckTime && (
                            <Chip
                                icon={<AccessTime />}
                                label={`Last update: ${format(lastCheckTime, 'HH:mm')}`}
                                size="small"
                                variant="outlined"
                            />
                        )}
                        <Tooltip title="Refresh data">
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
                                    {stat.trend && (
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
                            Weekly Trend
                        </Typography>
                        <ResponsiveContainer width="100%" height="85%">
                            <AreaChart data={weeklyTrend}>
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
                                />
                                <Area
                                    type="monotone"
                                    dataKey="clean"
                                    stackId="1"
                                    stroke={theme.palette.success.main}
                                    fillOpacity={1}
                                    fill="url(#colorClean)"
                                />
                            </AreaChart>
                        </ResponsiveContainer>
                    </Paper>
                </Grid>

                {/* Service Distribution */}
                <Grid item xs={12} md={4}>
                    <Paper sx={{ p: 3, height: 400 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            Service Distribution
                        </Typography>
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
                        <Box sx={{ display: 'flex', justifyContent: 'center', gap: 2, mt: 2 }}>
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
                    </Paper>
                </Grid>
            </Grid>

            {/* Recent Activity */}
            <Paper sx={{ p: 3 }}>
                <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                    Recent Activity
                </Typography>
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
                                label={activity.status}
                                size="small"
                                color={activity.status === 'spam' ? 'error' : 'success'}
                                icon={activity.status === 'spam' ? <Warning /> : <CheckCircle />}
                            />
                        </Box>
                    ))}
                </Box>
            </Paper>
        </Box>
    );
});

export default DashboardPage;