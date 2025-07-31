import React, { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { observer } from 'mobx-react-lite';
import {
    Box,
    Drawer,
    AppBar,
    Toolbar,
    List,
    Typography,
    Divider,
    IconButton,
    ListItem,
    ListItemIcon,
    ListItemText,
    ListItemButton,
    Avatar,
    Menu,
    MenuItem,
    Badge,
    Tooltip,
    useTheme,
    useMediaQuery,
    Chip,
} from '@mui/material';
import {
    Menu as MenuIcon,
    ChevronLeft,
    Dashboard,
    Phone,
    CheckCircle,
    People,
    Settings,
    BarChart,
    ExitToApp,
    AccountCircle,
    Notifications,
    Brightness4,
    Brightness7,
} from '@mui/icons-material';
import { authStore } from '../stores/AuthStore';

const drawerWidth = 280;

interface MenuItem {
    text: string;
    icon: React.ReactNode;
    path: string;
    roles?: string[];
    badge?: number;
}

const Layout: React.FC = observer(() => {
    const theme = useTheme();
    const navigate = useNavigate();
    const location = useLocation();
    const isMobile = useMediaQuery(theme.breakpoints.down('md'));

    const [open, setOpen] = useState(!isMobile);
    const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
    const [notificationAnchor, setNotificationAnchor] = useState<null | HTMLElement>(null);

    const menuItems: MenuItem[] = [
        {
            text: 'Dashboard',
            icon: <Dashboard />,
            path: '/dashboard',
        },
        {
            text: 'Phone Numbers',
            icon: <Phone />,
            path: '/phones',
        },
        {
            text: 'Checks',
            icon: <CheckCircle />,
            path: '/checks',
            badge: 3, // Example: show pending checks
        },
        {
            text: 'Statistics',
            icon: <BarChart />,
            path: '/statistics',
        },
        {
            text: 'Users',
            icon: <People />,
            path: '/users',
            roles: ['admin'],
        },
        {
            text: 'Settings',
            icon: <Settings />,
            path: '/settings',
            roles: ['admin', 'supervisor'],
        },
    ];

    const handleDrawerToggle = () => {
        setOpen(!open);
    };

    const handleProfileMenuOpen = (event: React.MouseEvent<HTMLElement>) => {
        setAnchorEl(event.currentTarget);
    };

    const handleProfileMenuClose = () => {
        setAnchorEl(null);
    };

    const handleLogout = () => {
        authStore.logout();
    };

    const isMenuItemVisible = (item: MenuItem) => {
        if (!item.roles) return true;
        return authStore.hasRole(item.roles);
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
        <Box sx={{ display: 'flex' }}>
            <AppBar
                position="fixed"
                sx={{
                    zIndex: theme.zIndex.drawer + 1,
                    backdropFilter: 'blur(20px)',
                    background: 'rgba(10, 14, 26, 0.8)',
                    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
                }}
            >
                <Toolbar>
                    <IconButton
                        color="inherit"
                        aria-label="toggle drawer"
                        onClick={handleDrawerToggle}
                        edge="start"
                        sx={{ mr: 2 }}
                    >
                        {open ? <ChevronLeft /> : <MenuIcon />}
                    </IconButton>

                    <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1, fontWeight: 600 }}>
                        SpamChecker
                    </Typography>

                    {/* Notifications */}
                    <Tooltip title="Notifications">
                        <IconButton
                            color="inherit"
                            onClick={(e) => setNotificationAnchor(e.currentTarget)}
                        >
                            <Badge badgeContent={5} color="error">
                                <Notifications />
                            </Badge>
                        </IconButton>
                    </Tooltip>

                    {/* Theme toggle */}
                    <Tooltip title="Toggle theme">
                        <IconButton color="inherit" sx={{ ml: 1 }}>
                            {theme.palette.mode === 'dark' ? <Brightness7 /> : <Brightness4 />}
                        </IconButton>
                    </Tooltip>

                    {/* Profile menu */}
                    <Box sx={{ ml: 2 }}>
                        <Tooltip title="Account settings">
                            <IconButton
                                onClick={handleProfileMenuOpen}
                                size="small"
                                sx={{ ml: 2 }}
                                aria-controls={Boolean(anchorEl) ? 'account-menu' : undefined}
                                aria-haspopup="true"
                                aria-expanded={Boolean(anchorEl) ? 'true' : undefined}
                            >
                                <Avatar sx={{ width: 36, height: 36, bgcolor: 'primary.main' }}>
                                    {authStore.user?.username.charAt(0).toUpperCase()}
                                </Avatar>
                            </IconButton>
                        </Tooltip>
                    </Box>
                </Toolbar>
            </AppBar>

            <Drawer
                variant={isMobile ? 'temporary' : 'persistent'}
                open={open}
                onClose={handleDrawerToggle}
                sx={{
                    width: drawerWidth,
                    flexShrink: 0,
                    '& .MuiDrawer-paper': {
                        width: drawerWidth,
                        boxSizing: 'border-box',
                        background: 'rgba(26, 31, 46, 0.95)',
                        backdropFilter: 'blur(20px)',
                        borderRight: '1px solid rgba(255, 255, 255, 0.1)',
                    },
                }}
            >
                <Toolbar />
                <Box sx={{ overflow: 'auto', flex: 1 }}>
                    {/* User info */}
                    <Box sx={{ p: 2, mb: 2 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                            <Avatar sx={{ width: 48, height: 48, bgcolor: 'primary.main', mr: 2 }}>
                                {authStore.user?.username.charAt(0).toUpperCase()}
                            </Avatar>
                            <Box>
                                <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                                    {authStore.user?.username}
                                </Typography>
                                <Chip
                                    label={authStore.user?.role}
                                    size="small"
                                    color={getRoleColor(authStore.user?.role || 'user')}
                                    sx={{ height: 20, fontSize: '0.75rem' }}
                                />
                            </Box>
                        </Box>
                        <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                            {authStore.user?.email}
                        </Typography>
                    </Box>

                    <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)' }} />

                    {/* Navigation */}
                    <List sx={{ px: 1, py: 2 }}>
                        {menuItems.filter(isMenuItemVisible).map((item) => (
                            <ListItem key={item.path} disablePadding sx={{ mb: 0.5 }}>
                                <ListItemButton
                                    onClick={() => navigate(item.path)}
                                    selected={location.pathname.startsWith(item.path)}
                                    sx={{
                                        borderRadius: 2,
                                        '&.Mui-selected': {
                                            backgroundColor: 'rgba(144, 202, 249, 0.1)',
                                            '&:hover': {
                                                backgroundColor: 'rgba(144, 202, 249, 0.15)',
                                            },
                                        },
                                        '&:hover': {
                                            backgroundColor: 'rgba(255, 255, 255, 0.05)',
                                        },
                                    }}
                                >
                                    <ListItemIcon sx={{ color: location.pathname.startsWith(item.path) ? 'primary.main' : 'inherit' }}>
                                        {item.badge ? (
                                            <Badge badgeContent={item.badge} color="error">
                                                {item.icon}
                                            </Badge>
                                        ) : (
                                            item.icon
                                        )}
                                    </ListItemIcon>
                                    <ListItemText
                                        primary={item.text}
                                        primaryTypographyProps={{
                                            fontWeight: location.pathname.startsWith(item.path) ? 600 : 400,
                                        }}
                                    />
                                </ListItemButton>
                            </ListItem>
                        ))}
                    </List>
                </Box>

                {/* Bottom section */}
                <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)' }} />
                <Box sx={{ p: 2 }}>
                    <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                        © 2024 SpamChecker
                    </Typography>
                </Box>
            </Drawer>

            {/* Main content */}
            <Box
                component="main"
                sx={{
                    flexGrow: 1,
                    p: 3,
                    width: { sm: `calc(100% - ${open ? drawerWidth : 0}px)` },
                    ml: { sm: open ? 0 : 0 },
                    transition: theme.transitions.create(['margin', 'width'], {
                        easing: theme.transitions.easing.sharp,
                        duration: theme.transitions.duration.leavingScreen,
                    }),
                }}
            >
                <Toolbar />
                <Outlet />
            </Box>

            {/* Profile menu */}
            <Menu
                anchorEl={anchorEl}
                id="account-menu"
                open={Boolean(anchorEl)}
                onClose={handleProfileMenuClose}
                onClick={handleProfileMenuClose}
                transformOrigin={{ horizontal: 'right', vertical: 'top' }}
                anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
                PaperProps={{
                    elevation: 0,
                    sx: {
                        overflow: 'visible',
                        filter: 'drop-shadow(0px 2px 8px rgba(0,0,0,0.32))',
                        mt: 1.5,
                        '& .MuiAvatar-root': {
                            width: 32,
                            height: 32,
                            ml: -0.5,
                            mr: 1,
                        },
                    },
                }}
            >
                <MenuItem onClick={() => navigate('/profile')}>
                    <Avatar>
                        <AccountCircle />
                    </Avatar>
                    My Profile
                </MenuItem>
                <Divider />
                <MenuItem onClick={handleLogout}>
                    <ListItemIcon>
                        <ExitToApp fontSize="small" />
                    </ListItemIcon>
                    Logout
                </MenuItem>
            </Menu>

            {/* Notifications menu */}
            <Menu
                anchorEl={notificationAnchor}
                open={Boolean(notificationAnchor)}
                onClose={() => setNotificationAnchor(null)}
                transformOrigin={{ horizontal: 'right', vertical: 'top' }}
                anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
                PaperProps={{
                    sx: {
                        width: 360,
                        maxHeight: 400,
                    },
                }}
            >
                <Box sx={{ p: 2 }}>
                    <Typography variant="h6" sx={{ mb: 2 }}>
                        Notifications
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                        No new notifications
                    </Typography>
                </Box>
            </Menu>
        </Box>
    );
});

export default Layout;