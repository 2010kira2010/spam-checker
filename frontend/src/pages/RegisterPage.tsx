import React, { useState } from 'react';
import { observer } from 'mobx-react-lite';
import { useNavigate, Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
    Box,
    Card,
    CardContent,
    TextField,
    Button,
    Typography,
    Alert,
    InputAdornment,
    IconButton,
    LinearProgress,
    Container,
    Stack,
    Divider,
    FormControl,
    Select,
    MenuItem,
} from '@mui/material';
import {
    Visibility,
    VisibilityOff,
    AccountCircle,
    Lock,
    Email,
    PhoneInTalk,
    Security,
    Speed,
    Language,
    AdminPanelSettings,
    SupervisorAccount,
    Person,
    ArrowBack,
} from '@mui/icons-material';
import { useSnackbar } from 'notistack';
import { authStore } from '../stores/AuthStore';

const RegisterPage: React.FC = observer(() => {
    const { t, i18n } = useTranslation();
    const navigate = useNavigate();
    const { enqueueSnackbar } = useSnackbar();
    const [showPassword, setShowPassword] = useState(false);
    const [showConfirmPassword, setShowConfirmPassword] = useState(false);
    const [formData, setFormData] = useState({
        username: '',
        email: '',
        password: '',
        confirmPassword: '',
        role: 'user' as 'admin' | 'supervisor' | 'user',
    });
    const [errors, setErrors] = useState<Record<string, string>>({});

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData({
            ...formData,
            [e.target.name]: e.target.value,
        });
        // Clear error for this field
        setErrors({
            ...errors,
            [e.target.name]: '',
        });
        authStore.clearError();
    };

    const handleRoleChange = (role: 'admin' | 'supervisor' | 'user') => {
        setFormData({
            ...formData,
            role,
        });
    };

    const validateForm = (): boolean => {
        const newErrors: Record<string, string> = {};

        // Username validation
        if (!formData.username) {
            newErrors.username = t('errors.requiredField');
        } else if (formData.username.length < 3) {
            newErrors.username = t('validation.usernameTooShort');
        } else if (formData.username.length > 50) {
            newErrors.username = t('validation.usernameTooLong');
        }

        // Email validation
        if (!formData.email) {
            newErrors.email = t('errors.requiredField');
        } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
            newErrors.email = t('errors.invalidEmail');
        }

        // Password validation
        if (!formData.password) {
            newErrors.password = t('errors.requiredField');
        } else if (formData.password.length < 6) {
            newErrors.password = t('users.passwordMinLength');
        }

        // Confirm password validation
        if (!formData.confirmPassword) {
            newErrors.confirmPassword = t('errors.requiredField');
        } else if (formData.password !== formData.confirmPassword) {
            newErrors.confirmPassword = t('users.passwordsDoNotMatch');
        }

        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!validateForm()) {
            return;
        }

        const success = await authStore.register({
            username: formData.username,
            email: formData.email,
            password: formData.password,
            role: formData.role,
        });

        if (success) {
            enqueueSnackbar(t('registration.success'), { variant: 'success' });
            navigate('/login');
        } else {
            enqueueSnackbar(authStore.error || t('registration.failed'), { variant: 'error' });
        }
    };

    const changeLanguage = (lng: string) => {
        i18n.changeLanguage(lng);
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

    const getRoleDescription = (role: string) => {
        return t(`registration.role.${role}Description`);
    };

    const features = [
        {
            icon: <PhoneInTalk sx={{ fontSize: 40 }} />,
            title: t('features.monitoring.title'),
            description: t('features.monitoring.description'),
        },
        {
            icon: <Security sx={{ fontSize: 40 }} />,
            title: t('features.detection.title'),
            description: t('features.detection.description'),
        },
        {
            icon: <Speed sx={{ fontSize: 40 }} />,
            title: t('features.realtime.title'),
            description: t('features.realtime.description'),
        },
    ];

    return (
        <Box
            sx={{
                minHeight: '100vh',
                display: 'flex',
                background: 'linear-gradient(135deg, #0a0e1a 0%, #1a1f2e 100%)',
                position: 'relative',
                overflow: 'hidden',
            }}
        >
            {/* Language selector */}
            <Box sx={{ position: 'absolute', top: 20, right: 20, zIndex: 1 }}>
                <FormControl size="small">
                    <Select
                        value={i18n.language}
                        onChange={(e) => changeLanguage(e.target.value)}
                        startAdornment={<Language sx={{ mr: 1, color: 'text.secondary' }} />}
                        sx={{
                            backgroundColor: 'rgba(255, 255, 255, 0.1)',
                            '& .MuiSelect-icon': { color: 'text.secondary' },
                        }}
                    >
                        <MenuItem value="en">English</MenuItem>
                        <MenuItem value="ru">Русский</MenuItem>
                    </Select>
                </FormControl>
            </Box>

            {/* Background decoration */}
            <Box
                sx={{
                    position: 'absolute',
                    top: -150,
                    right: -150,
                    width: 400,
                    height: 400,
                    borderRadius: '50%',
                    background: 'radial-gradient(circle, rgba(144, 202, 249, 0.1) 0%, transparent 70%)',
                    filter: 'blur(40px)',
                }}
            />
            <Box
                sx={{
                    position: 'absolute',
                    bottom: -150,
                    left: -150,
                    width: 400,
                    height: 400,
                    borderRadius: '50%',
                    background: 'radial-gradient(circle, rgba(244, 143, 177, 0.1) 0%, transparent 70%)',
                    filter: 'blur(40px)',
                }}
            />

            <Container maxWidth="lg" sx={{ display: 'flex', alignItems: 'center', py: 4 }}>
                <Box sx={{ flex: 1, display: 'flex', gap: 8 }}>
                    {/* Features section */}
                    <Box sx={{ flex: 1, display: { xs: 'none', md: 'block' } }}>
                        <Typography variant="h2" sx={{ mb: 2, fontWeight: 700 }}>
                            SpamChecker
                        </Typography>
                        <Typography variant="h5" sx={{ mb: 6, color: 'text.secondary' }}>
                            {t('registration.subtitle')}
                        </Typography>

                        <Stack spacing={4}>
                            {features.map((feature, index) => (
                                <Box key={index} sx={{ display: 'flex', gap: 3 }}>
                                    <Box
                                        sx={{
                                            width: 80,
                                            height: 80,
                                            borderRadius: 2,
                                            background: 'linear-gradient(135deg, rgba(144, 202, 249, 0.2) 0%, rgba(144, 202, 249, 0.1) 100%)',
                                            display: 'flex',
                                            alignItems: 'center',
                                            justifyContent: 'center',
                                            color: 'primary.main',
                                        }}
                                    >
                                        {feature.icon}
                                    </Box>
                                    <Box sx={{ flex: 1 }}>
                                        <Typography variant="h6" sx={{ mb: 1 }}>
                                            {feature.title}
                                        </Typography>
                                        <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                                            {feature.description}
                                        </Typography>
                                    </Box>
                                </Box>
                            ))}
                        </Stack>
                    </Box>

                    {/* Registration form */}
                    <Box sx={{ width: { xs: '100%', md: 480 } }}>
                        <Card
                            sx={{
                                backdropFilter: 'blur(20px)',
                                background: 'rgba(26, 31, 46, 0.8)',
                                border: '1px solid rgba(255, 255, 255, 0.1)',
                            }}
                        >
                            <CardContent sx={{ p: 4 }}>
                                <Button
                                    startIcon={<ArrowBack />}
                                    onClick={() => navigate('/login')}
                                    sx={{ mb: 2 }}
                                >
                                    {t('common.back')}
                                </Button>

                                <Typography variant="h4" sx={{ mb: 1, fontWeight: 600 }}>
                                    {t('registration.title')}
                                </Typography>
                                <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
                                    {t('registration.createAccount')}
                                </Typography>

                                {authStore.error && (
                                    <Alert severity="error" sx={{ mb: 3 }}>
                                        {authStore.error}
                                    </Alert>
                                )}

                                <form onSubmit={handleSubmit}>
                                    <TextField
                                        fullWidth
                                        name="username"
                                        label={t('auth.username')}
                                        value={formData.username}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
                                        autoFocus
                                        error={!!errors.username}
                                        helperText={errors.username}
                                        InputProps={{
                                            startAdornment: (
                                                <InputAdornment position="start">
                                                    <AccountCircle sx={{ color: 'text.secondary' }} />
                                                </InputAdornment>
                                            ),
                                        }}
                                    />

                                    <TextField
                                        fullWidth
                                        name="email"
                                        label={t('auth.email')}
                                        type="email"
                                        value={formData.email}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
                                        error={!!errors.email}
                                        helperText={errors.email}
                                        InputProps={{
                                            startAdornment: (
                                                <InputAdornment position="start">
                                                    <Email sx={{ color: 'text.secondary' }} />
                                                </InputAdornment>
                                            ),
                                        }}
                                    />

                                    <TextField
                                        fullWidth
                                        name="password"
                                        label={t('auth.password')}
                                        type={showPassword ? 'text' : 'password'}
                                        value={formData.password}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
                                        error={!!errors.password}
                                        helperText={errors.password || t('users.passwordMinLength')}
                                        InputProps={{
                                            startAdornment: (
                                                <InputAdornment position="start">
                                                    <Lock sx={{ color: 'text.secondary' }} />
                                                </InputAdornment>
                                            ),
                                            endAdornment: (
                                                <InputAdornment position="end">
                                                    <IconButton
                                                        onClick={() => setShowPassword(!showPassword)}
                                                        edge="end"
                                                    >
                                                        {showPassword ? <VisibilityOff /> : <Visibility />}
                                                    </IconButton>
                                                </InputAdornment>
                                            ),
                                        }}
                                    />

                                    <TextField
                                        fullWidth
                                        name="confirmPassword"
                                        label={t('users.confirmPassword')}
                                        type={showConfirmPassword ? 'text' : 'password'}
                                        value={formData.confirmPassword}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
                                        error={!!errors.confirmPassword}
                                        helperText={errors.confirmPassword}
                                        InputProps={{
                                            startAdornment: (
                                                <InputAdornment position="start">
                                                    <Lock sx={{ color: 'text.secondary' }} />
                                                </InputAdornment>
                                            ),
                                            endAdornment: (
                                                <InputAdornment position="end">
                                                    <IconButton
                                                        onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                                                        edge="end"
                                                    >
                                                        {showConfirmPassword ? <VisibilityOff /> : <Visibility />}
                                                    </IconButton>
                                                </InputAdornment>
                                            ),
                                        }}
                                    />

                                    {/* Role Selection */}
                                    <Box sx={{ mt: 3, mb: 2 }}>
                                        <Typography variant="subtitle2" sx={{ mb: 2, color: 'text.secondary' }}>
                                            {t('registration.selectRole')}
                                        </Typography>
                                        <Stack spacing={1}>
                                            {(['user', 'supervisor', 'admin'] as const).map((role) => (
                                                <Box
                                                    key={role}
                                                    onClick={() => handleRoleChange(role)}
                                                    sx={{
                                                        p: 2,
                                                        borderRadius: 2,
                                                        border: '1px solid',
                                                        borderColor: formData.role === role ? 'primary.main' : 'divider',
                                                        backgroundColor: formData.role === role ? 'action.selected' : 'transparent',
                                                        cursor: 'pointer',
                                                        transition: 'all 0.2s',
                                                        '&:hover': {
                                                            borderColor: 'primary.main',
                                                            backgroundColor: 'action.hover',
                                                        },
                                                    }}
                                                >
                                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                                        {getRoleIcon(role)}
                                                        <Box sx={{ flex: 1 }}>
                                                            <Typography variant="body1" sx={{ fontWeight: formData.role === role ? 600 : 400 }}>
                                                                {t(`users.${role}`)}
                                                            </Typography>
                                                            <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                                                                {getRoleDescription(role)}
                                                            </Typography>
                                                        </Box>
                                                    </Box>
                                                </Box>
                                            ))}
                                        </Stack>
                                    </Box>

                                    <Button
                                        type="submit"
                                        fullWidth
                                        variant="contained"
                                        size="large"
                                        disabled={authStore.isLoading}
                                        sx={{
                                            py: 1.5,
                                            mb: 2,
                                            mt: 3,
                                            background: 'linear-gradient(135deg, #42a5f5 0%, #90caf9 100%)',
                                            '&:hover': {
                                                background: 'linear-gradient(135deg, #1e88e5 0%, #64b5f6 100%)',
                                            },
                                        }}
                                    >
                                        {authStore.isLoading ? t('common.loading') : t('registration.register')}
                                    </Button>

                                    {authStore.isLoading && <LinearProgress sx={{ mb: 2 }} />}
                                </form>

                                <Divider sx={{ my: 3 }} />

                                <Box sx={{ textAlign: 'center' }}>
                                    <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                                        {t('registration.alreadyHaveAccount')}{' '}
                                        <Link to="/login" style={{ color: '#90caf9', textDecoration: 'none' }}>
                                            {t('auth.signIn')}
                                        </Link>
                                    </Typography>
                                </Box>
                            </CardContent>
                        </Card>
                    </Box>
                </Box>
            </Container>
        </Box>
    );
});

export default RegisterPage;