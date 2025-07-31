import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
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
    DialogActions,
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
    alpha,
} from '@mui/material';
import {
    Search,
    PlayArrow,
    Refresh,
    Image as ImageIcon,
    Phone,
    Warning,
    CheckCircle,
    AccessTime,
    FilterList,
    Close,
} from '@mui/icons-material';
import { format } from 'date-fns';
import axios from 'axios';
import { useSnackbar } from 'notistack';

interface CheckResult {
    id: number;
    phone_number_id: number;
    phone_number: string;
    service: {
        id: number;
        name: string;
        code: string;
    };
    is_spam: boolean;
    found_keywords: string[];
    checked_at: string;
    screenshot: string;
}

const ChecksPage: React.FC = observer(() => {
    const theme = useTheme();
    const { enqueueSnackbar } = useSnackbar();

    const [results, setResults] = useState<CheckResult[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [realtimeNumber, setRealtimeNumber] = useState('');
    const [realtimeLoading, setRealtimeLoading] = useState(false);
    const [realtimeResult, setRealtimeResult] = useState<any>(null);
    const [screenshotDialog, setScreenshotDialog] = useState<{
        open: boolean;
        url: string;
        title: string;
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
            setResults(response.data.results || []);
            setTotalCount(response.data.count || 0);
        } catch (error) {
            enqueueSnackbar('Failed to load check results', { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const handleRealtimeCheck = async () => {
        if (!realtimeNumber.trim()) {
            enqueueSnackbar('Please enter a phone number', { variant: 'warning' });
            return;
        }

        setRealtimeLoading(true);
        setRealtimeResult(null);

        try {
            const response = await axios.post('/checks/realtime', {
                phone_number: realtimeNumber,
            });

            setRealtimeResult(response.data);
            enqueueSnackbar('Real-time check completed', { variant: 'success' });
        } catch (error: any) {
            enqueueSnackbar(error.response?.data?.error || 'Failed to check number', { variant: 'error' });
        } finally {
            setRealtimeLoading(false);
        }
    };

    const handleCheckAll = async () => {
        if (!window.confirm('This will check all active phone numbers. Continue?')) {
            return;
        }

        try {
            await axios.post('/checks/all');
            enqueueSnackbar('Check started for all active phones', { variant: 'info' });
        } catch (error) {
            enqueueSnackbar('Failed to start check', { variant: 'error' });
        }
    };

    const handleViewScreenshot = (result: CheckResult) => {
        setScreenshotDialog({
            open: true,
            url: `/checks/screenshot/${result.id}`,
            title: `${result.phone_number} - ${result.service.name}`,
        });
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
                Phone Checks
            </Typography>

            {/* Real-time Check Card */}
            <Card sx={{ mb: 4 }}>
                <CardContent>
                    <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>
                        Real-time Check
                    </Typography>
                    <Grid container spacing={3} alignItems="flex-end">
                        <Grid item xs={12} md={6}>
                            <TextField
                                fullWidth
                                label="Phone Number"
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
                                {realtimeLoading ? 'Checking...' : 'Check Now'}
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
                                Check All Active
                            </Button>
                        </Grid>
                    </Grid>

                    {/* Real-time Results */}
                    {realtimeResult && (
                        <Box sx={{ mt: 3 }}>
                            <Alert severity="info" sx={{ mb: 2 }}>
                                Results for {realtimeResult.phone_number}
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
                                                    <Chip label="Error" size="small" color="error" />
                                                ) : result.is_spam ? (
                                                    <Chip label="Spam" size="small" color="warning" icon={<Warning />} />
                                                ) : (
                                                    <Chip label="Clean" size="small" color="success" icon={<CheckCircle />} />
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
                                                    No spam keywords found
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
                        Check History
                    </Typography>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                        <Tooltip title="Refresh">
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
                                <TableCell>Phone Number</TableCell>
                                <TableCell>Service</TableCell>
                                <TableCell>Status</TableCell>
                                <TableCell>Keywords Found</TableCell>
                                <TableCell>Checked At</TableCell>
                                <TableCell align="center">Actions</TableCell>
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
                                            No check results found
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
                                                    {result.phone_number}
                                                </Typography>
                                            </Box>
                                        </TableCell>
                                        <TableCell>
                                            <Chip label={result.service.name} size="small" variant="outlined" />
                                        </TableCell>
                                        <TableCell>
                                            {result.is_spam ? (
                                                <Chip
                                                    label="Spam"
                                                    size="small"
                                                    color="error"
                                                    icon={<Warning />}
                                                />
                                            ) : (
                                                <Chip
                                                    label="Clean"
                                                    size="small"
                                                    color="success"
                                                    icon={<CheckCircle />}
                                                />
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            {result.found_keywords.length > 0 ? (
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
                                                    None
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
                                            <Tooltip title="View Screenshot">
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
                onClose={() => setScreenshotDialog({ open: false, url: '', title: '' })}
                maxWidth="md"
                fullWidth
            >
                <DialogTitle>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Typography variant="h6">{screenshotDialog.title}</Typography>
                        <IconButton
                            onClick={() => setScreenshotDialog({ open: false, url: '', title: '' })}
                            size="small"
                        >
                            <Close />
                        </IconButton>
                    </Box>
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ textAlign: 'center', p: 2 }}>
                        <img
                            src={screenshotDialog.url}
                            alt="Screenshot"
                            style={{
                                maxWidth: '100%',
                                maxHeight: '70vh',
                                objectFit: 'contain',
                                borderRadius: 8,
                                boxShadow: theme.shadows[3],
                            }}
                        />
                    </Box>
                </DialogContent>
            </Dialog>
        </Box>
    );
});

export default ChecksPage;