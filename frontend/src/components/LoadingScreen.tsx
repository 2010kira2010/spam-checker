import React from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';

const LoadingScreen: React.FC = () => {
    return (
        <Box
            sx={{
                position: 'fixed',
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                backgroundColor: '#0a0e1a',
                zIndex: 9999,
            }}
        >
            <CircularProgress size={60} sx={{ mb: 3 }} />
            <Typography variant="h5" sx={{ fontWeight: 600, mb: 1 }}>
                SpamChecker
            </Typography>
            <Typography variant="body2" color="text.secondary">
                Loading...
            </Typography>
        </Box>
    );
};

export default LoadingScreen;