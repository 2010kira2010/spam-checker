import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
import { useTranslation } from 'react-i18next';
import {
    Box,
    Card,
    CardContent,
    Typography,
    Button,
    TextField,
    InputAdornment,
    Chip,
    Dialog,
    DialogTitle,
    DialogContent,
    Grid,
    Paper,
    IconButton,
    Tooltip,
    Alert,
    CircularProgress,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    TablePagination,
    useTheme,
} from '@mui/material';
import {
    PlayArrow,
    Refresh,
    Image as ImageIcon,
    Phone,
    Warning,
    CheckCircle,
    AccessTime,
    Close,
} from '@mui/icons-material';
import { format } from 'date-fns';
import axios from 'axios';
import { useSnackbar } from 'notistack';

interface PhoneNumber {
    id: number;
    number: string;
    description: string;
    is_active: boolean;
}

interface CheckResult {
    id: number;
    phone_number_id: number;
    phone_number?: string;
    service: {
        id: number;
        name: string;
        code: string;
    };
    is_spam: boolean;
    found_keywords: string[] | null;
    checked_at: string;
    screenshot: string;
}

interface CheckResultWithPhone extends CheckResult {
    phone: PhoneNumber;
}

const ChecksPage: React.FC = observer(() => {
    const { t } = useTranslation();
    const theme = useTheme();
    const { enqueueSnackbar } = useSnackbar();

    const [results, setResults] = useState<CheckResult[]>([]);
    const [phoneNumbers, setPhoneNumbers] = useState<Map<number, PhoneNumber>>(new Map());
    const [isLoading, setIsLoading] = useState(false);
    const [realtimeNumber, setRealtimeNumber] = useState('');
    const [realtimeLoading, setRealtimeLoading] = useState(false);
    const [realtimeResult, setRealtimeResult] = useState<any>(null);
    const [screenshotDialog, setScreenshotDialog] = useState<{
        open: boolean;
        url: string;
        title: string;
        loading?: boolean;
    }>({ open: false, url: '', title: '' });

    // Pagination
    const [page, setPage] = useState(0);
    const [rowsPerPage, setRowsPerPage] = useState(25);
    const [totalCount, setTotalCount] = useState(0);

    useEffect(() => {
        loadResults();
    }, [page, rowsPerPage]);

    const loadResults = async () => {
        setIsLoading(true);
        try {
            const response = await axios.get('/checks/results', {
                params: {
                    limit: rowsPerPage,
                    offset: page * rowsPerPage,
                },
            });

            const checkResults = response.data.results || [];
            setResults(checkResults);
            setTotalCount(response.data.count || 0);

            // Load phone numbers for the results
            const phoneIdsSet = new Set<number>();
            checkResults.forEach((r: CheckResult) => phoneIdsSet.add(r.phone_number_id));
            const phoneIds = Array.from(phoneIdsSet);
            if (phoneIds.length > 0) {
                await loadPhoneNumbers(phoneIds);
            }
        } catch (error) {
            enqueueSnackbar(t('errors.loadFailed'), { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const loadPhoneNumbers = async (phoneIds: number[]) => {
        try {
            // Load phone numbers in batches
            const phoneMap = new Map<number, PhoneNumber>();

            // Since we don't have a batch endpoint, we'll need to load them from the phones list
            // This is a workaround - ideally the API should return phone numbers with check results
            const phonesResponse = await axios.get('/phones', {
                params: {
                    limit: 100,
                    page: 1
                }
            });

            phonesResponse.data.phones.forEach((phone: PhoneNumber) => {
                if (phoneIds.includes(phone.id)) {
                    phoneMap.set(phone.id, phone);
                }
            });

            setPhoneNumbers(phoneMap);
        } catch (error) {
            console.error('Failed to load phone numbers:', error);
        }
    };

    const getPhoneNumber = (phoneId: number): string => {
        const phone = phoneNumbers.get(phoneId);
        return phone ? phone.number : `ID: ${phoneId}`;
    };

    const handleRealtimeCheck = async () => {
        if (!realtimeNumber.trim()) {
            enqueueSnackbar(t('errors.requiredField'), { variant: 'warning' });
            return;
        }

        setRealtimeLoading(true);
        setRealtimeResult(null);

        try {
            const response = await axios.post('/checks/realtime', {
                phone_number: realtimeNumber,
            });

            setRealtimeResult(response.data);
            enqueueSnackbar(t('notifications.checkCompleted'), { variant: 'success' });
        } catch (error: any) {
            enqueueSnackbar(error.response?.data?.error || t('errors.error'), { variant: 'error' });
        } finally {
            setRealtimeLoading(false);
        }
    };

    const handleCheckAll = async () => {
        if (!window.confirm(t('confirmations.deleteConfirmation'))) {
            return;
        }

        try {
            await axios.post('/checks/all');
            enqueueSnackbar(t('checks.checkStartedAllPhones'), { variant: 'info' });
        } catch (error) {
            enqueueSnackbar(t('errors.error'), { variant: 'error' });
        }
    };

    const handleViewScreenshot = async (result: CheckResult) => {
        const phoneNumber = getPhoneNumber(result.phone_number_id);
        setScreenshotDialog({
            open: true,
            url: '',
            title: `${phoneNumber} - ${result.service.name}`,
            loading: true,
        });

        try {
            // Load screenshot with authentication
            const response = await axios.get(`/checks/screenshot/${result.id}`, {
                responseType: 'blob',
            });

            // Create blob URL
            const imageUrl = URL.createObjectURL(response.data);

            setScreenshotDialog(prev => ({
                ...prev,
                url: imageUrl,
                loading: false,
            }));
        } catch (error) {
            console.error('Failed to load screenshot:', error);
            setScreenshotDialog(prev => ({
                ...prev,
                url: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="400" height="300"%3E%3Crect width="400" height="300" fill="%23333"%2F%3E%3Ctext x="50%25" y="50%25" dominant-baseline="middle" text-anchor="middle" fill="%23999" font-family="Arial" font-size="20"%3EScreenshot not available%3C%2Ftext%3E%3C%2Fsvg%3E',
                loading: false,
            }));
        }
    };

    const handleChangePage = (event: unknown, newPage: number) => {
        setPage(newPage);
    };

    const handleChangeRowsPerPage = (event: React.ChangeEvent<HTMLInputElement>) => {
        setRowsPerPage(parseInt(event.target.value, 10));
        setPage(0);
    };

    return (
        <Box>
            <Typography variant="h4" sx={{ mb: 3, fontWeight: 600 }}>
                {t('checks.title')}
            </Typography>

            {/* Real-time Check Card */}
            <Card sx={{ mb: 4 }}>
                <CardContent>
                    <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                        {t('checks.realtimeCheck')}
                    </Typography>
                    <Grid container spacing={3} alignItems="flex-end">
                        <Grid item xs={12} md={6}>
                            <TextField
                                fullWidth
                                label={t('checks.phoneNumber')}
                                placeholder="+7 (999) 123-45-67"
                                value={realtimeNumber}
                                onChange={(e) => setRealtimeNumber(e.target.value)}
                                InputProps={{
                                    startAdornment: (
                                        <InputAdornment position="start">
                                            <Phone />
                                        </InputAdornment>
                                    ),
                                }}
                                disabled={realtimeLoading}
                            />
                        </Grid>
                        <Grid item xs={12} md={3}>
                            <Button
                                fullWidth
                                variant="contained"
                                startIcon={realtimeLoading ? <CircularProgress size={20} /> : <PlayArrow />}
                                onClick={handleRealtimeCheck}
                                disabled={realtimeLoading}
                            >
                                {realtimeLoading ? t('checks.checking') : t('checks.checkNow')}
                            </Button>
                        </Grid>
                        <Grid item xs={12} md={3}>
                            <Button
                                fullWidth
                                variant="outlined"
                                color="warning"
                                startIcon={<PlayArrow />}
                                onClick={handleCheckAll}
                            >
                                {t('checks.checkAllActive')}
                            </Button>
                        </Grid>
                    </Grid>

                    {/* Real-time Results */}
                    {realtimeResult && (
                        <Box sx={{ mt: 3 }}>
                            <Alert severity="info" sx={{ mb: 2 }}>
                                {t('checks.resultsFor', { number: realtimeResult.phone_number })}
                            </Alert>
                            <Grid container spacing={2}>
                                {realtimeResult.results?.map((result: any, index: number) => (
                                    <Grid item xs={12} md={4} key={index}>
                                        <Paper
                                            sx={{
                                                p: 2,
                                                border: `1px solid ${
                                                    result.error
                                                        ? theme.palette.error.main
                                                        : result.is_spam
                                                            ? theme.palette.warning.main
                                                            : theme.palette.success.main
                                                }`,
                                            }}
                                        >
                                            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
                                                <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                                                    {result.service}
                                                </Typography>
                                                {result.error ? (
                                                    <Chip label={t('common.error')} size="small" color="error" />
                                                ) : result.is_spam ? (
                                                    <Chip label={t('phones.spam')} size="small" color="warning" icon={<Warning />} />
                                                ) : (
                                                    <Chip label={t('phones.clean')} size="small" color="success" icon={<CheckCircle />} />
                                                )}
                                            </Box>
                                            {result.error ? (
                                                <Typography variant="body2" color="error">
                                                    {result.error}
                                                </Typography>
                                            ) : result.found_keywords && result.found_keywords.length > 0 ? (
                                                <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                                                    {result.found_keywords.map((keyword: string, i: number) => (
                                                        <Chip key={i} label={keyword} size="small" variant="outlined" />
                                                    ))}
                                                </Box>
                                            ) : (
                                                <Typography variant="body2" color="text.secondary">
                                                    {t('checks.noSpamKeywords')}
                                                </Typography>
                                            )}
                                        </Paper>
                                    </Grid>
                                ))}
                            </Grid>
                        </Box>
                    )}
                </CardContent>
            </Card>

            {/* Results Table */}
            <Paper>
                <Box sx={{ p: 2, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <Typography variant="h6" sx={{ fontWeight: 600 }}>
                        {t('checks.checkHistory')}
                    </Typography>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                        <Tooltip title={t('common.refresh')}>
                            <IconButton onClick={loadResults} disabled={isLoading}>
                                <Refresh />
                            </IconButton>
                        </Tooltip>
                    </Box>
                </Box>

                <TableContainer>
                    <Table>
                        <TableHead>
                            <TableRow>
                                <TableCell>{t('checks.phoneNumber')}</TableCell>
                                <TableCell>{t('checks.service')}</TableCell>
                                <TableCell>{t('checks.status')}</TableCell>
                                <TableCell>{t('checks.keywordsFound')}</TableCell>
                                <TableCell>{t('checks.checkedAt')}</TableCell>
                                <TableCell align="center">{t('common.actions')}</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {isLoading ? (
                                <TableRow>
                                    <TableCell colSpan={6} align="center">
                                        <CircularProgress />
                                    </TableCell>
                                </TableRow>
                            ) : results.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={6} align="center">
                                        <Typography variant="body2" color="text.secondary">
                                            {t('checks.noCheckResults')}
                                        </Typography>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                results.map((result) => (
                                    <TableRow key={result.id}>
                                        <TableCell>
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                <Phone sx={{ fontSize: 18, color: 'text.secondary' }} />
                                                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                                    {getPhoneNumber(result.phone_number_id)}
                                                </Typography>
                                            </Box>
                                        </TableCell>
                                        <TableCell>
                                            <Chip label={result.service.name} size="small" variant="outlined" />
                                        </TableCell>
                                        <TableCell>
                                            {result.is_spam ? (
                                                <Chip
                                                    label={t('phones.spam')}
                                                    size="small"
                                                    color="error"
                                                    icon={<Warning />}
                                                />
                                            ) : (
                                                <Chip
                                                    label={t('phones.clean')}
                                                    size="small"
                                                    color="success"
                                                    icon={<CheckCircle />}
                                                />
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            {result.found_keywords && result.found_keywords.length > 0 ? (
                                                <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                                                    {result.found_keywords.slice(0, 3).map((keyword, i) => (
                                                        <Chip
                                                            key={i}
                                                            label={keyword}
                                                            size="small"
                                                            variant="outlined"
                                                            color="error"
                                                        />
                                                    ))}
                                                    {result.found_keywords.length > 3 && (
                                                        <Chip
                                                            label={`+${result.found_keywords.length - 3}`}
                                                            size="small"
                                                            variant="outlined"
                                                        />
                                                    )}
                                                </Box>
                                            ) : (
                                                <Typography variant="caption" color="text.secondary">
                                                    —
                                                </Typography>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                <AccessTime sx={{ fontSize: 16, color: 'text.secondary' }} />
                                                <Typography variant="caption">
                                                    {format(new Date(result.checked_at), 'MMM dd, HH:mm')}
                                                </Typography>
                                            </Box>
                                        </TableCell>
                                        <TableCell align="center">
                                            <Tooltip title={t('checks.viewScreenshot')}>
                                                <IconButton
                                                    size="small"
                                                    onClick={() => handleViewScreenshot(result)}
                                                >
                                                    <ImageIcon />
                                                </IconButton>
                                            </Tooltip>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </TableContainer>

                <TablePagination
                    rowsPerPageOptions={[10, 25, 50, 100]}
                    component="div"
                    count={totalCount}
                    rowsPerPage={rowsPerPage}
                    page={page}
                    onPageChange={handleChangePage}
                    onRowsPerPageChange={handleChangeRowsPerPage}
                />
            </Paper>

            {/* Screenshot Dialog */}
            <Dialog
                open={screenshotDialog.open}
                onClose={() => {
                    // Clean up blob URL when closing
                    if (screenshotDialog.url && screenshotDialog.url.startsWith('blob:')) {
                        URL.revokeObjectURL(screenshotDialog.url);
                    }
                    setScreenshotDialog({ open: false, url: '', title: '' });
                }}
                maxWidth="md"
                fullWidth
            >
                <DialogTitle>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Typography variant="h6">{screenshotDialog.title}</Typography>
                        <IconButton
                            onClick={() => {
                                if (screenshotDialog.url && screenshotDialog.url.startsWith('blob:')) {
                                    URL.revokeObjectURL(screenshotDialog.url);
                                }
                                setScreenshotDialog({ open: false, url: '', title: '' });
                            }}
                            size="small"
                        >
                            <Close />
                        </IconButton>
                    </Box>
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ textAlign: 'center', p: 2 }}>
                        {screenshotDialog.loading ? (
                            <CircularProgress />
                        ) : (
                            <img
                                src={screenshotDialog.url}
                                alt={t('checks.screenshot')}
                                style={{
                                    maxWidth: '100%',
                                    maxHeight: '70vh',
                                    objectFit: 'contain',
                                    borderRadius: 8,
                                    boxShadow: theme.shadows[3],
                                }}
                            />
                        )}
                    </Box>
                </DialogContent>
            </Dialog>
        </Box>
    );
});

export default ChecksPage;