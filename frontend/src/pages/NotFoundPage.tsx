import React from 'react';
import { useNavigate } from 'react-router-dom';
import {
    Box,
    Typography,
    Button,
    Container,
    useTheme,
    alpha,
} from '@mui/material';
import {
    Home,
    ArrowBack,
    SearchOff,
} from '@mui/icons-material';

const NotFoundPage: React.FC = () => {
    const theme = useTheme();
    const navigate = useNavigate();

    return (
        <Box
            sx={{
                minHeight: '80vh',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                position: 'relative',
                overflow: 'hidden',
            }}
        >
            {/* Background decoration */}
            <Box
                sx={{
                    position: 'absolute',
                    top: '50%',
                    left: '50%',
                    transform: 'translate(-50%, -50%)',
                    width: 600,
                    height: 600,
                    borderRadius: '50%',
                    background: `radial-gradient(circle, ${alpha(theme.palette.primary.main, 0.1)} 0%, transparent 70%)`,
                    filter: 'blur(60px)',
                }}
            />

            <Container maxWidth="sm">
                <Box sx={{ textAlign: 'center', position: 'relative', zIndex: 1 }}>
                    {/* Icon */}
                    <Box
                        sx={{
                            width: 120,
                            height: 120,
                            margin: '0 auto',
                            mb: 4,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            borderRadius: '50%',
                            background: `linear-gradient(135deg, ${alpha(theme.palette.primary.main, 0.2)} 0%, ${alpha(
                                theme.palette.primary.main,
                                0.1
                            )} 100%)`,
                            border: `2px solid ${alpha(theme.palette.primary.main, 0.3)}`,
                        }}
                    >
                        <SearchOff sx={{ fontSize: 60, color: theme.palette.primary.main }} />
                    </Box>

                    {/* 404 Text */}
                    <Typography
                        variant="h1"
                        sx={{
                            fontSize: { xs: '6rem', md: '8rem' },
                            fontWeight: 900,
                            background: `linear-gradient(135deg, ${theme.palette.primary.main} 0%, ${theme.palette.secondary.main} 100%)`,
                            backgroundClip: 'text',
                            WebkitBackgroundClip: 'text',
                            WebkitTextFillColor: 'transparent',
                            mb: 2,
                        }}
                    >
                        404
                    </Typography>

                    <Typography
                        variant="h4"
                        sx={{
                            mb: 2,
                            fontWeight: 600,
                        }}
                    >
                        Page Not Found
                    </Typography>

                    <Typography
                        variant="body1"
                        sx={{
                            mb: 4,
                            color: 'text.secondary',
                            maxWidth: 400,
                            mx: 'auto',
                        }}
                    >
                        The page you're looking for doesn't exist or has been moved.
                        Please check the URL or navigate back to the dashboard.
                    </Typography>

                    {/* Action Buttons */}
                    <Box sx={{ display: 'flex', gap: 2, justifyContent: 'center' }}>
                        <Button
                            variant="outlined"
                            startIcon={<ArrowBack />}
                            onClick={() => navigate(-1)}
                            sx={{
                                borderRadius: 2,
                                px: 3,
                            }}
                        >
                            Go Back
                        </Button>
                        <Button
                            variant="contained"
                            startIcon={<Home />}
                            onClick={() => navigate('/dashboard')}
                            sx={{
                                borderRadius: 2,
                                px: 3,
                                background: `linear-gradient(135deg, ${theme.palette.primary.main} 0%, ${theme.palette.primary.dark} 100%)`,
                                '&:hover': {
                                    background: `linear-gradient(135deg, ${theme.palette.primary.dark} 0%, ${theme.palette.primary.main} 100%)`,
                                },
                            }}
                        >
                            Go to Dashboard
                        </Button>
                    </Box>

                    {/* Additional Links */}
                    <Box sx={{ mt: 6 }}>
                        <Typography variant="body2" color="text.secondary">
                            Quick Links:
                        </Typography>
                        <Box sx={{ display: 'flex', gap: 2, justifyContent: 'center', mt: 1 }}>
                            <Button
                                size="small"
                                onClick={() => navigate('/phones')}
                                sx={{ textDecoration: 'underline' }}
                            >
                                Phone Numbers
                            </Button>
                            <Button
                                size="small"
                                onClick={() => navigate('/checks')}
                                sx={{ textDecoration: 'underline' }}
                            >
                                Checks
                            </Button>
                            <Button
                                size="small"
                                onClick={() => navigate('/settings')}
                                sx={{ textDecoration: 'underline' }}
                            >
                                Settings
                            </Button>
                        </Box>
                    </Box>
                </Box>
            </Container>
        </Box>
    );
};

export default NotFoundPage;