import React, { useEffect, useState } from 'react';
import { observer } from 'mobx-react-lite';
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
        if (window.confirm(`Are you sure you want to delete ${phone.number}?`)) {
            const success = await phoneStore.deletePhone(phone.id);
            if (success) {
                enqueueSnackbar('Phone deleted successfully', { variant: 'success' });
            } else {
                enqueueSnackbar(phoneStore.error || 'Failed to delete phone', { variant: 'error' });
            }
        }
        setAnchorEl(null);
    };

    const handleSubmit = async () => {
        if (editingPhone) {
            const success = await phoneStore.updatePhone(editingPhone.id, formData);
            if (success) {
                enqueueSnackbar('Phone updated successfully', { variant: 'success' });
                setOpenDialog(false);
            } else {
                enqueueSnackbar(phoneStore.error || 'Failed to update phone', { variant: 'error' });
            }
        } else {
            const success = await phoneStore.createPhone(formData);
            if (success) {
                enqueueSnackbar('Phone created successfully', { variant: 'success' });
                setOpenDialog(false);
            } else {
                enqueueSnackbar(phoneStore.error || 'Failed to create phone', { variant: 'error' });
            }
        }
    };

    const handleCheckPhone = async (phone: any) => {
        const success = await phoneStore.checkPhone(phone.id);
        if (success) {
            enqueueSnackbar(`Check started for ${phone.number}`, { variant: 'info' });
        } else {
            enqueueSnackbar(phoneStore.error || 'Failed to start check', { variant: 'error' });
        }
    };

    const handleCheckAll = async () => {
        const success = await phoneStore.checkAllPhones();
        if (success) {
            enqueueSnackbar('Check started for all active phones', { variant: 'info' });
        } else {
            enqueueSnackbar(phoneStore.error || 'Failed to start check', { variant: 'error' });
        }
    };

    const handleImport = async () => {
        if (!selectedFile) return;

        const result = await phoneStore.importPhones(selectedFile);
        if (result.success) {
            enqueueSnackbar(`Imported ${result.imported} phones successfully`, { variant: 'success' });
            if (result.errors && result.errors.length > 0) {
                enqueueSnackbar(`${result.errors.length} errors occurred during import`, { variant: 'warning' });
            }
            setImportDialogOpen(false);
            setSelectedFile(null);
        } else {
            enqueueSnackbar('Failed to import phones', { variant: 'error' });
        }
    };

    const handleExport = async () => {
        const success = await phoneStore.exportPhones();
        if (success) {
            enqueueSnackbar('Phones exported successfully', { variant: 'success' });
        } else {
            enqueueSnackbar('Failed to export phones', { variant: 'error' });
        }
    };

    const columns: GridColDef[] = [
        {
            field: 'number',
            headerName: 'Phone Number',
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
            headerName: 'Description',
            flex: 1.5,
            minWidth: 200,
        },
        {
            field: 'is_active',
            headerName: 'Status',
            width: 120,
            renderCell: (params: GridRenderCellParams) => (
                <Chip
                    label={params.value ? 'Active' : 'Inactive'}
                    size="small"
                    color={params.value ? 'success' : 'default'}
                    icon={params.value ? <CheckCircle /> : <Cancel />}
                />
            ),
        },
        {
            field: 'check_results',
            headerName: 'Last Check',
            width: 150,
            renderCell: (params: GridRenderCellParams) => {
                const results = params.value as any[];
                if (!results || results.length === 0) {
                    return <Typography variant="caption" color="text.secondary">Never checked</Typography>;
                }
                const lastResult = results[0];
                const isSpam = lastResult.is_spam;
                return (
                    <Chip
                        label={isSpam ? 'Spam' : 'Clean'}
                        size="small"
                        color={isSpam ? 'error' : 'success'}
                        icon={isSpam ? <Warning /> : <CheckCircle />}
                    />
                );
            },
        },
        {
            field: 'created_at',
            headerName: 'Created',
            width: 150,
            renderCell: (params: GridRenderCellParams) => (
                <Typography variant="caption">
                    {format(new Date(params.value), 'dd MMM yyyy')}
                </Typography>
            ),
        },
        {
            field: 'actions',
            headerName: 'Actions',
            width: 120,
            sortable: false,
            renderCell: (params: GridRenderCellParams) => (
                <Box>
                    <Tooltip title="Check now">
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
                    Phone Numbers
                </Typography>

                {/* Actions Bar */}
                <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
                    <TextField
                        placeholder="Search phones..."
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
                                Add Phone
                            </Button>
                            <Button
                                variant="outlined"
                                startIcon={<FileUpload />}
                                onClick={() => setImportDialogOpen(true)}
                            >
                                Import
                            </Button>
                        </>
                    )}

                    <Button
                        variant="outlined"
                        startIcon={<FileDownload />}
                        onClick={handleExport}
                    >
                        Export
                    </Button>

                    {authStore.isAdmin && (
                        <Button
                            variant="outlined"
                            color="warning"
                            startIcon={<PlayArrow />}
                            onClick={handleCheckAll}
                        >
                            Check All
                        </Button>
                    )}
                </Box>

                {/* Stats */}
                <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
                    <Chip label={`Total: ${phoneStore.totalItems}`} />
                    <Chip label={`Active: ${phoneStore.phones.filter(p => p.is_active).length}`} color="success" />
                    <Chip label={`Inactive: ${phoneStore.phones.filter(p => !p.is_active).length}`} />
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
                    {editingPhone ? 'Edit Phone' : 'Add Phone'}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
                        <TextField
                            label="Phone Number"
                            value={formData.number}
                            onChange={(e) => setFormData({ ...formData, number: e.target.value })}
                            fullWidth
                            required
                            helperText="Format: +7XXXXXXXXXX"
                        />
                        <TextField
                            label="Description"
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
                            label="Active"
                        />
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setOpenDialog(false)}>Cancel</Button>
                    <Button onClick={handleSubmit} variant="contained">
                        {editingPhone ? 'Update' : 'Create'}
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Import Dialog */}
            <Dialog open={importDialogOpen} onClose={() => setImportDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>Import Phones</DialogTitle>
                <DialogContent>
                    <Alert severity="info" sx={{ mb: 2 }}>
                        CSV file should have columns: number, description
                    </Alert>
                    <input
                        type="file"
                        accept=".csv"
                        onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                        style={{ marginTop: 16 }}
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setImportDialogOpen(false)}>Cancel</Button>
                    <Button onClick={handleImport} variant="contained" disabled={!selectedFile}>
                        Import
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
                        Edit
                    </MenuItem>
                )}
                {authStore.isAdmin && (
                    <MenuItem onClick={() => handleDeletePhone(selectedPhone)} sx={{ color: 'error.main' }}>
                        <Delete sx={{ mr: 1, fontSize: 20 }} />
                        Delete
                    </MenuItem>
                )}
            </Menu>
        </Box>
    );
});

export default PhonesPage;