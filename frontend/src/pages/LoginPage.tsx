import React, { useState } from 'react';
import { observer } from 'mobx-react-lite';
import { useNavigate } from 'react-router-dom';
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
} from '@mui/material';
import {
    Visibility,
    VisibilityOff,
    AccountCircle,
    Lock,
    PhoneInTalk,
    Security,
    Speed,
} from '@mui/icons-material';
import { useSnackbar } from 'notistack';
import { authStore } from '../stores/AuthStore';

const LoginPage: React.FC = observer(() => {
    const navigate = useNavigate();
    const { enqueueSnackbar } = useSnackbar();
    const [showPassword, setShowPassword] = useState(false);
    const [formData, setFormData] = useState({
        login: '',
        password: '',
    });

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData({
            ...formData,
            [e.target.name]: e.target.value,
        });
        authStore.clearError();
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        const success = await authStore.login(formData);

        if (success) {
            enqueueSnackbar('Login successful!', { variant: 'success' });
            navigate('/dashboard');
        } else {
            enqueueSnackbar(authStore.error || 'Login failed', { variant: 'error' });
        }
    };

    const features = [
        {
            icon: <PhoneInTalk sx={{ fontSize: 40 }} />,
            title: 'Phone Number Monitoring',
            description: 'Track and monitor your company phone numbers across multiple spam services',
        },
        {
            icon: <Security sx={{ fontSize: 40 }} />,
            title: 'Multi-Service Detection',
            description: 'Check numbers in Yandex АОН, Kaspersky Who Calls, and GetContact',
        },
        {
            icon: <Speed sx={{ fontSize: 40 }} />,
            title: 'Real-time Analysis',
            description: 'Get instant notifications when your numbers are marked as spam',
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
                            Protect your business reputation
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

                    {/* Login form */}
                    <Box sx={{ width: { xs: '100%', md: 420 } }}>
                        <Card
                            sx={{
                                backdropFilter: 'blur(20px)',
                                background: 'rgba(26, 31, 46, 0.8)',
                                border: '1px solid rgba(255, 255, 255, 0.1)',
                            }}
                        >
                            <CardContent sx={{ p: 4 }}>
                                <Typography variant="h4" sx={{ mb: 1, fontWeight: 600 }}>
                                    Welcome Back
                                </Typography>
                                <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
                                    Sign in to continue to SpamChecker
                                </Typography>

                                {authStore.error && (
                                    <Alert severity="error" sx={{ mb: 3 }}>
                                        {authStore.error}
                                    </Alert>
                                )}

                                <form onSubmit={handleSubmit}>
                                    <TextField
                                        fullWidth
                                        name="login"
                                        label="Email or Username"
                                        value={formData.login}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
                                        autoFocus
                                        InputProps={{
                                            startAdornment: (
                                                <InputAdornment position="start">
                                                    <AccountCircle sx={{ color: 'text.secondary' }} />
                                                </InputAdornment>
                                            ),
                                        }}
                                        sx={{ mb: 2 }}
                                    />

                                    <TextField
                                        fullWidth
                                        name="password"
                                        label="Password"
                                        type={showPassword ? 'text' : 'password'}
                                        value={formData.password}
                                        onChange={handleChange}
                                        margin="normal"
                                        required
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
                                        sx={{ mb: 4 }}
                                    />

                                    <Button
                                        type="submit"
                                        fullWidth
                                        variant="contained"
                                        size="large"
                                        disabled={authStore.isLoading}
                                        sx={{
                                            py: 1.5,
                                            mb: 2,
                                            background: 'linear-gradient(135deg, #42a5f5 0%, #90caf9 100%)',
                                            '&:hover': {
                                                background: 'linear-gradient(135deg, #1e88e5 0%, #64b5f6 100%)',
                                            },
                                        }}
                                    >
                                        {authStore.isLoading ? 'Signing in...' : 'Sign In'}
                                    </Button>

                                    {authStore.isLoading && <LinearProgress sx={{ mb: 2 }} />}
                                </form>

                                <Divider sx={{ my: 3 }}>
                                    <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                                        DEMO CREDENTIALS
                                    </Typography>
                                </Divider>

                                <Box sx={{ textAlign: 'center' }}>
                                    <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                                        Email: admin@spamchecker.com
                                    </Typography>
                                    <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                                        Password: admin123
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

export default LoginPage;