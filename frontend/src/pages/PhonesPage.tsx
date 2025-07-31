import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
import { useTranslation } from 'react-i18next';
import {
    Box,
    Button,
    Typography,
    IconButton,
    Tooltip,
    TextField,
    InputAdornment,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    FormControlLabel,
    Switch,
    Chip,
    Menu,
    MenuItem,
    LinearProgress,
    Alert,
    Paper,
} from '@mui/material';
import { DataGrid, GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import {
    Add,
    Search,
    FileUpload,
    FileDownload,
    Edit,
    Delete,
    MoreVert,
    Phone,
    CheckCircle,
    Cancel,
    PlayArrow,
    Warning,
} from '@mui/icons-material';
import { format } from 'date-fns';
import { useSnackbar } from 'notistack';
import { phoneStore } from '../stores/PhoneStore';
import { authStore } from '../stores/AuthStore';

interface PhoneFormData {
    number: string;
    description: string;
    is_active: boolean;
}

const PhonesPage: React.FC = observer(() => {
    const { t } = useTranslation();
    const { enqueueSnackbar } = useSnackbar();
    const [searchQuery, setSearchQuery] = useState('');
    const [openDialog, setOpenDialog] = useState(false);
    const [editingPhone, setEditingPhone] = useState<any>(null);
    const [formData, setFormData] = useState<PhoneFormData>({
        number: '',
        description: '',
        is_active: true,
    });
    const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
    const [selectedPhone, setSelectedPhone] = useState<any>(null);
    const [importDialogOpen, setImportDialogOpen] = useState(false);
    const [selectedFile, setSelectedFile] = useState<File | null>(null);

    useEffect(() => {
        phoneStore.fetchPhones();
    }, []);

    const handleSearch = (value: string) => {
        setSearchQuery(value);
        phoneStore.setSearchQuery(value);
    };

    const handleAddPhone = () => {
        setEditingPhone(null);
        setFormData({
            number: '',
            description: '',
            is_active: true,
        });
        setOpenDialog(true);
    };

    const handleEditPhone = (phone: any) => {
        setEditingPhone(phone);
        setFormData({
            number: phone.number,
            description: phone.description,
            is_active: phone.is_active,
        });
        setOpenDialog(true);
        setAnchorEl(null);
    };

    const handleDeletePhone = async (phone: any) => {
        if (window.confirm(t('phones.confirmDelete', { number: phone.number }))) {
            const success = await phoneStore.deletePhone(phone.id);
            if (success) {
                enqueueSnackbar(t('phones.phoneDeleted'), { variant: 'success' });
            } else {
                enqueueSnackbar(phoneStore.error || t('errors.deleteFailed'), { variant: 'error' });
            }
        }
        setAnchorEl(null);
    };

    const handleSubmit = async () => {
        if (editingPhone) {
            const success = await phoneStore.updatePhone(editingPhone.id, formData);
            if (success) {
                enqueueSnackbar(t('phones.phoneUpdated'), { variant: 'success' });
                setOpenDialog(false);
            } else {
                enqueueSnackbar(phoneStore.error || t('errors.updateFailed'), { variant: 'error' });
            }
        } else {
            const success = await phoneStore.createPhone(formData);
            if (success) {
                enqueueSnackbar(t('phones.phoneCreated'), { variant: 'success' });
                setOpenDialog(false);
            } else {
                enqueueSnackbar(phoneStore.error || t('errors.saveFailed'), { variant: 'error' });
            }
        }
    };

    const handleCheckPhone = async (phone: any) => {
        const success = await phoneStore.checkPhone(phone.id);
        if (success) {
            enqueueSnackbar(t('phones.checkStarted', { number: phone.number }), { variant: 'info' });
        } else {
            enqueueSnackbar(phoneStore.error || t('errors.error'), { variant: 'error' });
        }
    };

    const handleCheckAll = async () => {
        const success = await phoneStore.checkAllPhones();
        if (success) {
            enqueueSnackbar(t('checks.checkStartedAllPhones'), { variant: 'info' });
        } else {
            enqueueSnackbar(phoneStore.error || t('errors.error'), { variant: 'error' });
        }
    };

    const handleImport = async () => {
        if (!selectedFile) return;

        const result = await phoneStore.importPhones(selectedFile);
        if (result.success) {
            enqueueSnackbar(t('phones.importSuccess', { count: result.imported }), { variant: 'success' });
            if (result.errors && result.errors.length > 0) {
                enqueueSnackbar(`${result.errors.length} ${t('errors.error')}`, { variant: 'warning' });
            }
            setImportDialogOpen(false);
            setSelectedFile(null);
        } else {
            enqueueSnackbar(t('errors.importFailed'), { variant: 'error' });
        }
    };

    const handleExport = async () => {
        const success = await phoneStore.exportPhones();
        if (success) {
            enqueueSnackbar(t('phones.exportSuccess'), { variant: 'success' });
        } else {
            enqueueSnackbar(t('errors.exportFailed'), { variant: 'error' });
        }
    };

    const columns: GridColDef[] = [
        {
            field: 'number',
            headerName: t('phones.phoneNumber'),
            flex: 1,
            minWidth: 150,
            renderCell: (params: GridRenderCellParams) => (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Phone sx={{ fontSize: 18, color: 'text.secondary' }} />
                    <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                        {params.value}
                    </Typography>
                </Box>
            ),
        },
        {
            field: 'description',
            headerName: t('phones.description'),
            flex: 1.5,
            minWidth: 200,
        },
        {
            field: 'is_active',
            headerName: t('common.status'),
            width: 120,
            renderCell: (params: GridRenderCellParams) => (
                <Chip
                    label={params.value ? t('common.active') : t('common.inactive')}
                    size="small"
                    color={params.value ? 'success' : 'default'}
                    icon={params.value ? <CheckCircle /> : <Cancel />}
                />
            ),
        },
        {
            field: 'check_results',
            headerName: t('phones.lastCheck'),
            width: 150,
            renderCell: (params: GridRenderCellParams) => {
                const results = params.value as any[];
                if (!results || results.length === 0) {
                    return <Typography variant="caption" color="text.secondary">{t('phones.neverChecked')}</Typography>;
                }
                const lastResult = results[0];
                const isSpam = lastResult.is_spam;
                return (
                    <Chip
                        label={isSpam ? t('phones.spam') : t('phones.clean')}
                        size="small"
                        color={isSpam ? 'error' : 'success'}
                        icon={isSpam ? <Warning /> : <CheckCircle />}
                    />
                );
            },
        },
        {
            field: 'created_at',
            headerName: t('users.created'),
            width: 150,
            renderCell: (params: GridRenderCellParams) => (
                <Typography variant="caption">
                    {format(new Date(params.value), 'dd MMM yyyy')}
                </Typography>
            ),
        },
        {
            field: 'actions',
            headerName: t('common.actions'),
            width: 120,
            sortable: false,
            renderCell: (params: GridRenderCellParams) => (
                <Box>
                    <Tooltip title={t('phones.checkNow')}>
                        <IconButton
                            size="small"
                            onClick={() => handleCheckPhone(params.row)}
                            sx={{ mr: 1 }}
                        >
                            <PlayArrow />
                        </IconButton>
                    </Tooltip>
                    <IconButton
                        size="small"
                        onClick={(e) => {
                            setAnchorEl(e.currentTarget);
                            setSelectedPhone(params.row);
                        }}
                    >
                        <MoreVert />
                    </IconButton>
                </Box>
            ),
        },
    ];

    const canEdit = authStore.hasRole(['admin', 'supervisor']);

    return (
        <Box>
            {/* Header */}
            <Box sx={{ mb: 4 }}>
                <Typography variant="h4" sx={{ mb: 3, fontWeight: 600 }}>
                    {t('phones.title')}
                </Typography>

                {/* Actions Bar */}
                <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
                    <TextField
                        placeholder={`${t('common.search')}...`}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        size="small"
                        sx={{ flex: 1, minWidth: 250 }}
                        InputProps={{
                            startAdornment: (
                                <InputAdornment position="start">
                                    <Search />
                                </InputAdornment>
                            ),
                        }}
                    />

                    {canEdit && (
                        <>
                            <Button
                                variant="contained"
                                startIcon={<Add />}
                                onClick={handleAddPhone}
                            >
                                {t('phones.addPhone')}
                            </Button>
                            <Button
                                variant="outlined"
                                startIcon={<FileUpload />}
                                onClick={() => setImportDialogOpen(true)}
                            >
                                {t('common.import')}
                            </Button>
                        </>
                    )}

                    <Button
                        variant="outlined"
                        startIcon={<FileDownload />}
                        onClick={handleExport}
                    >
                        {t('common.export')}
                    </Button>

                    {authStore.isAdmin && (
                        <Button
                            variant="outlined"
                            color="warning"
                            startIcon={<PlayArrow />}
                            onClick={handleCheckAll}
                        >
                            {t('phones.checkAll')}
                        </Button>
                    )}
                </Box>

                {/* Stats */}
                <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
                    <Chip label={`${t('phones.total')}: ${phoneStore.totalItems}`} />
                    <Chip label={`${t('phones.active')}: ${phoneStore.phones.filter(p => p.is_active).length}`} color="success" />
                    <Chip label={`${t('phones.inactive')}: ${phoneStore.phones.filter(p => !p.is_active).length}`} />
                </Box>
            </Box>

            {/* Data Grid */}
            <Paper sx={{ height: 600 }}>
                {phoneStore.isLoading && <LinearProgress />}
                <DataGrid
                    rows={phoneStore.phones}
                    columns={columns}
                    pageSizeOptions={[10, 20, 50]}
                    paginationModel={{
                        pageSize: phoneStore.pageSize,
                        page: phoneStore.currentPage - 1,
                    }}
                    onPaginationModelChange={(model) => {
                        if (model.pageSize !== phoneStore.pageSize) {
                            phoneStore.setPageSize(model.pageSize);
                        }
                        if (model.page !== phoneStore.currentPage - 1) {
                            phoneStore.setPage(model.page + 1);
                        }
                    }}
                    rowCount={phoneStore.totalItems}
                    paginationMode="server"
                    loading={phoneStore.isLoading}
                    disableRowSelectionOnClick
                    sx={{
                        border: 'none',
                        '& .MuiDataGrid-cell': {
                            borderBottom: `1px solid rgba(255, 255, 255, 0.05)`,
                        },
                        '& .MuiDataGrid-columnHeaders': {
                            backgroundColor: 'background.paper',
                            borderBottom: `2px solid rgba(255, 255, 255, 0.1)`,
                        },
                    }}
                />
            </Paper>

            {/* Phone Form Dialog */}
            <Dialog open={openDialog} onClose={() => setOpenDialog(false)} maxWidth="sm" fullWidth>
                <DialogTitle>
                    {editingPhone ? t('phones.editPhone') : t('phones.addPhone')}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
                        <TextField
                            label={t('phones.phoneNumber')}
                            value={formData.number}
                            onChange={(e) => setFormData({ ...formData, number: e.target.value })}
                            fullWidth
                            required
                            helperText="Format: +7XXXXXXXXXX"
                        />
                        <TextField
                            label={t('phones.description')}
                            value={formData.description}
                            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                            fullWidth
                            multiline
                            rows={3}
                        />
                        <FormControlLabel
                            control={
                                <Switch
                                    checked={formData.is_active}
                                    onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                                />
                            }
                            label={t('common.active')}
                        />
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setOpenDialog(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSubmit} variant="contained">
                        {editingPhone ? t('common.update') : t('common.create')}
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Import Dialog */}
            <Dialog open={importDialogOpen} onClose={() => setImportDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>{t('phones.importPhones')}</DialogTitle>
                <DialogContent>
                    <Alert severity="info" sx={{ mb: 2 }}>
                        {t('phones.importInstructions')}
                    </Alert>
                    <input
                        type="file"
                        accept=".csv"
                        onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                        style={{ marginTop: 16 }}
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setImportDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleImport} variant="contained" disabled={!selectedFile}>
                        {t('common.import')}
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Actions Menu */}
            <Menu
                anchorEl={anchorEl}
                open={Boolean(anchorEl)}
                onClose={() => setAnchorEl(null)}
            >
                {canEdit && (
                    <MenuItem onClick={() => handleEditPhone(selectedPhone)}>
                        <Edit sx={{ mr: 1, fontSize: 20 }} />
                        {t('common.edit')}
                    </MenuItem>
                )}
                {authStore.isAdmin && (
                    <MenuItem onClick={() => handleDeletePhone(selectedPhone)} sx={{ color: 'error.main' }}>
                        <Delete sx={{ mr: 1, fontSize: 20 }} />
                        {t('common.delete')}
                    </MenuItem>
                )}
            </Menu>
        </Box>
    );
});

export default PhonesPage;