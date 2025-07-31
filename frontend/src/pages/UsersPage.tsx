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
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Select,
    MenuItem,
    FormControl,
    InputLabel,
    Chip,
    IconButton,
    Avatar,
    Switch,
    FormControlLabel,
    Alert,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Paper,
    Tooltip,
    InputAdornment,
} from '@mui/material';
import {
    Edit,
    Delete,
    PersonAdd,
    AccountCircle,
    Email,
    Lock,
    AdminPanelSettings,
    SupervisorAccount,
    Person,
    CheckCircle,
    Cancel,
} from '@mui/icons-material';
import { format } from 'date-fns';
import axios from 'axios';
import { useSnackbar } from 'notistack';
import { authStore } from '../stores/AuthStore';

interface User {
    id: number;
    username: string;
    email: string;
    role: 'admin' | 'supervisor' | 'user';
    is_active: boolean;
    created_at: string;
    updated_at: string;
}

interface UserFormData {
    username: string;
    email: string;
    password: string;
    role: 'admin' | 'supervisor' | 'user';
    is_active: boolean;
}

const UsersPage: React.FC = observer(() => {
    const { t } = useTranslation();
    const { enqueueSnackbar } = useSnackbar();
    const [users, setUsers] = useState<User[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [dialogOpen, setDialogOpen] = useState(false);
    const [editingUser, setEditingUser] = useState<User | null>(null);
    const [formData, setFormData] = useState<UserFormData>({
        username: '',
        email: '',
        password: '',
        role: 'user',
        is_active: true,
    });
    const [changePasswordDialog, setChangePasswordDialog] = useState(false);
    const [passwordData, setPasswordData] = useState({
        userId: 0,
        newPassword: '',
        confirmPassword: '',
    });

    useEffect(() => {
        loadUsers();
    }, []);

    const loadUsers = async () => {
        setIsLoading(true);
        try {
            const response = await axios.get('/users');
            setUsers(response.data.users || []);
        } catch (error) {
            enqueueSnackbar(t('errors.loadFailed'), { variant: 'error' });
        } finally {
            setIsLoading(false);
        }
    };

    const handleAddUser = () => {
        setEditingUser(null);
        setFormData({
            username: '',
            email: '',
            password: '',
            role: 'user',
            is_active: true,
        });
        setDialogOpen(true);
    };

    const handleEditUser = (user: User) => {
        setEditingUser(user);
        setFormData({
            username: user.username,
            email: user.email,
            password: '',
            role: user.role,
            is_active: user.is_active,
        });
        setDialogOpen(true);
    };

    const handleDeleteUser = async (user: User) => {
        if (user.id === authStore.user?.id) {
            enqueueSnackbar(t('users.cannotDeleteSelf'), { variant: 'error' });
            return;
        }

        if (!window.confirm(t('users.confirmDelete', { username: user.username }))) {
            return;
        }

        try {
            await axios.delete(`/users/${user.id}`);
            enqueueSnackbar(t('users.userDeleted'), { variant: 'success' });
            loadUsers();
        } catch (error) {
            enqueueSnackbar(t('errors.deleteFailed'), { variant: 'error' });
        }
    };

    const handleSubmit = async () => {
        if (!formData.username || !formData.email) {
            enqueueSnackbar(t('errors.requiredField'), { variant: 'error' });
            return;
        }

        if (!editingUser && !formData.password) {
            enqueueSnackbar(t('errors.requiredField'), { variant: 'error' });
            return;
        }

        try {
            if (editingUser) {
                await axios.put(`/users/${editingUser.id}`, formData);
                enqueueSnackbar(t('users.userUpdated'), { variant: 'success' });
            } else {
                await axios.post('/users', formData);
                enqueueSnackbar(t('users.userCreated'), { variant: 'success' });
            }
            setDialogOpen(false);
            loadUsers();
        } catch (error: any) {
            enqueueSnackbar(error.response?.data?.error || t('errors.saveFailed'), { variant: 'error' });
        }
    };

    const handleChangePassword = async () => {
        if (passwordData.newPassword !== passwordData.confirmPassword) {
            enqueueSnackbar(t('users.passwordsDoNotMatch'), { variant: 'error' });
            return;
        }

        if (passwordData.newPassword.length < 6) {
            enqueueSnackbar(t('users.passwordMinLength'), { variant: 'error' });
            return;
        }

        try {
            await axios.put(`/users/${passwordData.userId}/password`, {
                password: passwordData.newPassword,
            });
            enqueueSnackbar(t('users.passwordChanged'), { variant: 'success' });
            setChangePasswordDialog(false);
            setPasswordData({ userId: 0, newPassword: '', confirmPassword: '' });
        } catch (error) {
            enqueueSnackbar(t('errors.updateFailed'), { variant: 'error' });
        }
    };

    const getRoleIcon = (role: string) => {
        switch (role) {
            case 'admin':
                return <AdminPanelSettings />;
            case 'supervisor':
                return <SupervisorAccount />;
            default:
                return <Person />;
        }
    };

    const getRoleColor = (role: string) => {
        switch (role) {
            case 'admin':
                return 'error';
            case 'supervisor':
                return 'warning';
            default:
                return 'info';
        }
    };

    return (
        <Box>
            <Box sx={{ mb: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="h4" sx={{ fontWeight: 600 }}>
                    {t('users.title')}
                </Typography>
                <Button
                    variant="contained"
                    startIcon={<PersonAdd />}
                    onClick={handleAddUser}
                >
                    {t('users.addUser')}
                </Button>
            </Box>

            {/* Stats Cards */}
            <Box sx={{ display: 'flex', gap: 2, mb: 4 }}>
                <Card sx={{ flex: 1 }}>
                    <CardContent>
                        <Typography variant="h4" sx={{ fontWeight: 700 }}>
                            {users.length}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                            {t('users.totalUsers')}
                        </Typography>
                    </CardContent>
                </Card>
                <Card sx={{ flex: 1 }}>
                    <CardContent>
                        <Typography variant="h4" sx={{ fontWeight: 700 }}>
                            {users.filter(u => u.is_active).length}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                            {t('users.activeUsers')}
                        </Typography>
                    </CardContent>
                </Card>
                <Card sx={{ flex: 1 }}>
                    <CardContent>
                        <Typography variant="h4" sx={{ fontWeight: 700 }}>
                            {users.filter(u => u.role === 'admin').length}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                            {t('users.administrators')}
                        </Typography>
                    </CardContent>
                </Card>
            </Box>

            {/* Users Table */}
            <TableContainer component={Paper}>
                <Table>
                    <TableHead>
                        <TableRow>
                            <TableCell>{t('users.user')}</TableCell>
                            <TableCell>{t('users.email')}</TableCell>
                            <TableCell>{t('users.role')}</TableCell>
                            <TableCell>{t('users.status')}</TableCell>
                            <TableCell>{t('users.created')}</TableCell>
                            <TableCell align="right">{t('common.actions')}</TableCell>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {users.map((user) => (
                            <TableRow key={user.id}>
                                <TableCell>
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                        <Avatar sx={{ bgcolor: 'primary.main' }}>
                                            {user.username.charAt(0).toUpperCase()}
                                        </Avatar>
                                        <Typography variant="body1" sx={{ fontWeight: 500 }}>
                                            {user.username}
                                        </Typography>
                                    </Box>
                                </TableCell>
                                <TableCell>
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                        <Email sx={{ fontSize: 18, color: 'text.secondary' }} />
                                        {user.email}
                                    </Box>
                                </TableCell>
                                <TableCell>
                                    <Chip
                                        icon={getRoleIcon(user.role)}
                                        label={t(`users.${user.role}`)}
                                        size="small"
                                        color={getRoleColor(user.role)}
                                    />
                                </TableCell>
                                <TableCell>
                                    <Chip
                                        icon={user.is_active ? <CheckCircle /> : <Cancel />}
                                        label={user.is_active ? t('users.active') : t('users.inactive')}
                                        size="small"
                                        color={user.is_active ? 'success' : 'default'}
                                    />
                                </TableCell>
                                <TableCell>
                                    <Typography variant="caption">
                                        {format(new Date(user.created_at), 'MMM dd, yyyy')}
                                    </Typography>
                                </TableCell>
                                <TableCell align="right">
                                    <Tooltip title={t('users.changePassword')}>
                                        <IconButton
                                            size="small"
                                            onClick={() => {
                                                setPasswordData({ userId: user.id, newPassword: '', confirmPassword: '' });
                                                setChangePasswordDialog(true);
                                            }}
                                        >
                                            <Lock />
                                        </IconButton>
                                    </Tooltip>
                                    <Tooltip title={t('common.edit')}>
                                        <IconButton size="small" onClick={() => handleEditUser(user)}>
                                            <Edit />
                                        </IconButton>
                                    </Tooltip>
                                    <Tooltip title={t('common.delete')}>
                                        <IconButton
                                            size="small"
                                            onClick={() => handleDeleteUser(user)}
                                            disabled={user.id === authStore.user?.id}
                                            color="error"
                                        >
                                            <Delete />
                                        </IconButton>
                                    </Tooltip>
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>

            {/* User Dialog */}
            <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>
                    {editingUser ? t('users.editUser') : t('users.addUser')}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
                        <TextField
                            label={t('auth.username')}
                            value={formData.username}
                            onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                            fullWidth
                            required
                            InputProps={{
                                startAdornment: (
                                    <InputAdornment position="start">
                                        <AccountCircle />
                                    </InputAdornment>
                                ),
                            }}
                        />
                        <TextField
                            label={t('auth.email')}
                            type="email"
                            value={formData.email}
                            onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                            fullWidth
                            required
                            InputProps={{
                                startAdornment: (
                                    <InputAdornment position="start">
                                        <Email />
                                    </InputAdornment>
                                ),
                            }}
                        />
                        {!editingUser && (
                            <TextField
                                label={t('auth.password')}
                                type="password"
                                value={formData.password}
                                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                fullWidth
                                required
                                helperText={t('users.passwordMinLength')}
                                InputProps={{
                                    startAdornment: (
                                        <InputAdornment position="start">
                                            <Lock />
                                        </InputAdornment>
                                    ),
                                }}
                            />
                        )}
                        <FormControl fullWidth>
                            <InputLabel>{t('users.role')}</InputLabel>
                            <Select
                                value={formData.role}
                                label={t('users.role')}
                                onChange={(e) => setFormData({ ...formData, role: e.target.value as any })}
                            >
                                <MenuItem value="user">{t('users.user')}</MenuItem>
                                <MenuItem value="supervisor">{t('users.supervisor')}</MenuItem>
                                <MenuItem value="admin">{t('users.admin')}</MenuItem>
                            </Select>
                        </FormControl>
                        <FormControlLabel
                            control={
                                <Switch
                                    checked={formData.is_active}
                                    onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                                />
                            }
                            label={t('common.active')}
                        />
                        {editingUser && (
                            <Alert severity="info">
                                {t('users.changePassword')}
                            </Alert>
                        )}
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleSubmit} variant="contained">
                        {editingUser ? t('common.update') : t('common.create')}
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Change Password Dialog */}
            <Dialog
                open={changePasswordDialog}
                onClose={() => setChangePasswordDialog(false)}
                maxWidth="xs"
                fullWidth
            >
                <DialogTitle>{t('users.changePassword')}</DialogTitle>
                <DialogContent>
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
                        <TextField
                            label={t('users.newPassword')}
                            type="password"
                            value={passwordData.newPassword}
                            onChange={(e) => setPasswordData({ ...passwordData, newPassword: e.target.value })}
                            fullWidth
                            required
                        />
                        <TextField
                            label={t('users.confirmPassword')}
                            type="password"
                            value={passwordData.confirmPassword}
                            onChange={(e) => setPasswordData({ ...passwordData, confirmPassword: e.target.value })}
                            fullWidth
                            required
                        />
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setChangePasswordDialog(false)}>{t('common.cancel')}</Button>
                    <Button onClick={handleChangePassword} variant="contained">
                        {t('users.changePassword')}
                    </Button>
                </DialogActions>
            </Dialog>
        </Box>
    );
});

export default UsersPage;