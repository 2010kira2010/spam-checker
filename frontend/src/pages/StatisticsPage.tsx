import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
import {
    Box,
    Card,
    CardContent,
    Typography,
    Grid,
    Paper,
    Select,
    MenuItem,
    FormControl,
    InputLabel,
    ToggleButton,
    ToggleButtonGroup,
    Chip,
    LinearProgress,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    useTheme,
    alpha,
} from '@mui/material';
import {
    BarChart,
    Bar,
    LineChart,
    Line,
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    ResponsiveContainer,
    RadarChart,
    PolarGrid,
    PolarAngleAxis,
    PolarRadiusAxis,
    Radar,
} from 'recharts';
import { format } from 'date-fns';
import {
    TrendingUp,
    TrendingDown,
    Warning,
    Phone,
    AccessTime,
    BarChart as BarChartIcon,
} from '@mui/icons-material';
import axios from 'axios';
import { useSnackbar } from 'notistack';

interface TimeSeriesData {
    date: string;
    total_checks: number;
    spam_count: number;
    clean_count: number;
    spam_rate: number;
}

interface ServiceStats {
    service_name: string;
    total_checks: number;
    spam_count: number;
    spam_rate: number;
}

interface KeywordStats {
    keyword: string;
    count: number;
}

const StatisticsPage: React.FC = observer(() => {
    const theme = useTheme();
    const { enqueueSnackbar } = useSnackbar();
    const [isLoading, setIsLoading] = useState(true);
    const [timeRange, setTimeRange] = useState(7);
    const [viewType, setViewType] = useState<'line' | 'bar' | 'area'>('area');

    const [overviewStats, setOverviewStats] = useState({
        total_phones: 0,
        active_phones: 0,
        total_checks: 0,
        spam_detections: 0,
        spam_rate: 0,
        active_gateways: 0,
    });

    const [timeSeriesData, setTimeSeriesData] = useState<TimeSeriesData[]>([]);
    const [serviceStats, setServiceStats] = useState<ServiceStats[]>([]);
    const [topKeywords, setTopKeywords] = useState<KeywordStats[]>([]);
    const [recentSpam, setRecentSpam] = useState<any[]>([]);

    useEffect(() => {
        loadStatistics();
    }, [timeRange]);

    const loadStatistics = async () => {
        setIsLoading(true);
        try {
            // Load all statistics data
            const [overview, timeSeries, services, keywords, recent] = await Promise.all([
                axios.get('/statistics/overview'),
                axios.get(`/statistics/timeseries?days=${timeRange}`),
                axios.get('/statistics/services'),
                axios.get('/statistics/keywords?limit=10'),
                axios.get('/statistics/recent-spam?limit=10'),
            ]);

            setOverviewStats(overview.data);
            setTimeSeriesData(timeSeries.data);
            setServiceStats(services.data);
            setTopKeywords(keywords.data);
            setRecentSpam(recent.data);
        } catch (error) {
            enqueueSnackbar('Failed to load statistics', { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const pieColors = ['#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8'];

    const formatNumber = (num: number) => {
        return new Intl.NumberFormat('en-US', {
            notation: 'compact',
            maximumFractionDigits: 1,
        }).format(num);
    };

    const statCards = [
        {
            title: 'Total Checks',
            value: formatNumber(overviewStats.total_checks),
            icon: <BarChartIcon />,
            color: theme.palette.primary.main,
            trend: 15,
        },
        {
            title: 'Spam Detected',
            value: formatNumber(overviewStats.spam_detections),
            icon: <Warning />,
            color: theme.palette.error.main,
            trend: -8,
        },
        {
            title: 'Spam Rate',
            value: `${overviewStats.spam_rate.toFixed(1)}%`,
            icon: <TrendingUp />,
            color: theme.palette.warning.main,
        },
        {
            title: 'Active Phones',
            value: formatNumber(overviewStats.active_phones),
            icon: <Phone />,
            color: theme.palette.success.main,
            trend: 5,
        },
    ];

    // Prepare radar chart data for services
    const radarData = serviceStats.map(service => ({
        service: service.service_name,
        checks: (service.total_checks / Math.max(...serviceStats.map(s => s.total_checks))) * 100,
        spam: service.spam_rate,
    }));

    return (
        <Box>
            <Box sx={{ mb: 4 }}>
                <Typography variant="h4" sx={{ mb: 3, fontWeight: 600 }}>
                    Statistics & Analytics
                </Typography>

                {/* Controls */}
                <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
                    <FormControl size="small" sx={{ minWidth: 120 }}>
                        <InputLabel>Time Range</InputLabel>
                        <Select
                            value={timeRange}
                            label="Time Range"
                            onChange={(e) => setTimeRange(e.target.value as number)}
                        >
                            <MenuItem value={7}>Last 7 days</MenuItem>
                            <MenuItem value={14}>Last 14 days</MenuItem>
                            <MenuItem value={30}>Last 30 days</MenuItem>
                            <MenuItem value={90}>Last 90 days</MenuItem>
                        </Select>
                    </FormControl>

                    <ToggleButtonGroup
                        value={viewType}
                        exclusive
                        onChange={(e, value) => value && setViewType(value)}
                        size="small"
                    >
                        <ToggleButton value="line">Line</ToggleButton>
                        <ToggleButton value="bar">Bar</ToggleButton>
                        <ToggleButton value="area">Area</ToggleButton>
                    </ToggleButtonGroup>
                </Box>
            </Box>

            {isLoading && <LinearProgress sx={{ mb: 2 }} />}

            {/* Stat Cards */}
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
                                        <Chip
                                            size="small"
                                            icon={stat.trend > 0 ? <TrendingUp /> : <TrendingDown />}
                                            label={`${Math.abs(stat.trend)}%`}
                                            color={stat.trend > 0 ? 'success' : 'error'}
                                            sx={{ height: 24 }}
                                        />
                                    )}
                                </Box>
                                <Typography variant="h4" sx={{ fontWeight: 700, mb: 0.5 }}>
                                    {stat.value}
                                </Typography>
                                <Typography variant="body2" color="text.secondary">
                                    {stat.title}
                                </Typography>
                            </CardContent>
                        </Card>
                    </Grid>
                ))}
            </Grid>

            {/* Time Series Chart */}
            <Grid container spacing={3} sx={{ mb: 4 }}>
                <Grid item xs={12}>
                    <Paper sx={{ p: 3 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            Check Trends Over Time
                        </Typography>
                        <ResponsiveContainer width="100%" height={400}>
                            {viewType === 'line' ? (
                                <LineChart data={timeSeriesData}>
                                    <CartesianGrid strokeDasharray="3 3" stroke={alpha(theme.palette.divider, 0.3)} />
                                    <XAxis dataKey="date" stroke={theme.palette.text.secondary} />
                                    <YAxis stroke={theme.palette.text.secondary} />
                                    <Tooltip
                                        contentStyle={{
                                            backgroundColor: theme.palette.background.paper,
                                            border: `1px solid ${theme.palette.divider}`,
                                            borderRadius: 8,
                                        }}
                                    />
                                    <Legend />
                                    <Line
                                        type="monotone"
                                        dataKey="spam_count"
                                        stroke={theme.palette.error.main}
                                        strokeWidth={2}
                                        name="Spam"
                                    />
                                    <Line
                                        type="monotone"
                                        dataKey="clean_count"
                                        stroke={theme.palette.success.main}
                                        strokeWidth={2}
                                        name="Clean"
                                    />
                                </LineChart>
                            ) : viewType === 'bar' ? (
                                <BarChart data={timeSeriesData}>
                                    <CartesianGrid strokeDasharray="3 3" stroke={alpha(theme.palette.divider, 0.3)} />
                                    <XAxis dataKey="date" stroke={theme.palette.text.secondary} />
                                    <YAxis stroke={theme.palette.text.secondary} />
                                    <Tooltip
                                        contentStyle={{
                                            backgroundColor: theme.palette.background.paper,
                                            border: `1px solid ${theme.palette.divider}`,
                                            borderRadius: 8,
                                        }}
                                    />
                                    <Legend />
                                    <Bar dataKey="spam_count" fill={theme.palette.error.main} name="Spam" />
                                    <Bar dataKey="clean_count" fill={theme.palette.success.main} name="Clean" />
                                </BarChart>
                            ) : (
                                <AreaChart data={timeSeriesData}>
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
                                    <XAxis dataKey="date" stroke={theme.palette.text.secondary} />
                                    <YAxis stroke={theme.palette.text.secondary} />
                                    <Tooltip
                                        contentStyle={{
                                            backgroundColor: theme.palette.background.paper,
                                            border: `1px solid ${theme.palette.divider}`,
                                            borderRadius: 8,
                                        }}
                                    />
                                    <Legend />
                                    <Area
                                        type="monotone"
                                        dataKey="spam_count"
                                        stackId="1"
                                        stroke={theme.palette.error.main}
                                        fillOpacity={1}
                                        fill="url(#colorSpam)"
                                        name="Spam"
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="clean_count"
                                        stackId="1"
                                        stroke={theme.palette.success.main}
                                        fillOpacity={1}
                                        fill="url(#colorClean)"
                                        name="Clean"
                                    />
                                </AreaChart>
                            )}
                        </ResponsiveContainer>
                    </Paper>
                </Grid>
            </Grid>

            {/* Service Stats */}
            <Grid container spacing={3} sx={{ mb: 4 }}>
                <Grid item xs={12} md={6}>
                    <Paper sx={{ p: 3, height: 400 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            Service Performance
                        </Typography>
                        <ResponsiveContainer width="100%" height="85%">
                            <RadarChart data={radarData}>
                                <PolarGrid stroke={alpha(theme.palette.divider, 0.3)} />
                                <PolarAngleAxis dataKey="service" stroke={theme.palette.text.secondary} />
                                <PolarRadiusAxis stroke={theme.palette.text.secondary} />
                                <Radar
                                    name="Check Volume"
                                    dataKey="checks"
                                    stroke={theme.palette.primary.main}
                                    fill={theme.palette.primary.main}
                                    fillOpacity={0.6}
                                />
                                <Radar
                                    name="Spam Rate"
                                    dataKey="spam"
                                    stroke={theme.palette.error.main}
                                    fill={theme.palette.error.main}
                                    fillOpacity={0.6}
                                />
                                <Legend />
                            </RadarChart>
                        </ResponsiveContainer>
                    </Paper>
                </Grid>

                <Grid item xs={12} md={6}>
                    <Paper sx={{ p: 3, height: 400 }}>
                        <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                            Top Spam Keywords
                        </Typography>
                        <ResponsiveContainer width="100%" height="85%">
                            <BarChart data={topKeywords} layout="horizontal">
                                <CartesianGrid strokeDasharray="3 3" stroke={alpha(theme.palette.divider, 0.3)} />
                                <XAxis type="number" stroke={theme.palette.text.secondary} />
                                <YAxis dataKey="keyword" type="category" stroke={theme.palette.text.secondary} />
                                <Tooltip
                                    contentStyle={{
                                        backgroundColor: theme.palette.background.paper,
                                        border: `1px solid ${theme.palette.divider}`,
                                        borderRadius: 8,
                                    }}
                                />
                                <Bar dataKey="count" fill={theme.palette.warning.main} />
                            </BarChart>
                        </ResponsiveContainer>
                    </Paper>
                </Grid>
            </Grid>

            {/* Recent Spam Detections */}
            <Paper sx={{ p: 3 }}>
                <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                    Recent Spam Detections
                </Typography>
                <TableContainer>
                    <Table>
                        <TableHead>
                            <TableRow>
                                <TableCell>Phone Number</TableCell>
                                <TableCell>Service</TableCell>
                                <TableCell>Keywords Found</TableCell>
                                <TableCell>Detected At</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {recentSpam.map((detection, index) => (
                                <TableRow key={index}>
                                    <TableCell>
                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                            <Phone sx={{ fontSize: 18, color: 'text.secondary' }} />
                                            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                                {detection.phone_number}
                                            </Typography>
                                        </Box>
                                    </TableCell>
                                    <TableCell>
                                        <Chip label={detection.service_name} size="small" />
                                    </TableCell>
                                    <TableCell>
                                        <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                                            {detection.found_keywords?.map((keyword: string, i: number) => (
                                                <Chip
                                                    key={i}
                                                    label={keyword}
                                                    size="small"
                                                    variant="outlined"
                                                    color="error"
                                                />
                                            ))}
                                        </Box>
                                    </TableCell>
                                    <TableCell>
                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                            <AccessTime sx={{ fontSize: 16, color: 'text.secondary' }} />
                                            <Typography variant="caption">
                                                {format(new Date(detection.checked_at), 'MMM dd, HH:mm')}
                                            </Typography>
                                        </Box>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
            </Paper>
        </Box>
    );
});

export default StatisticsPage;