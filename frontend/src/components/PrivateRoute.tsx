import React from 'react';
import { Navigate } from 'react-router-dom';
import { observer } from 'mobx-react-lite';
import { authStore } from '../stores/AuthStore';

interface PrivateRouteProps {
    children: React.ReactNode;
    requiredRoles?: string[];
}

const PrivateRoute: React.FC<PrivateRouteProps> = observer(({ children, requiredRoles }) => {
    // Check if user is authenticated
    if (!authStore.isAuthenticated) {
        return <Navigate to="/login" replace />;
    }

    // Check if user has required roles
    if (requiredRoles && requiredRoles.length > 0) {
        if (!authStore.hasRole(requiredRoles)) {
            return <Navigate to="/dashboard" replace />;
        }
    }

    return <>{children}</>;
});

export default PrivateRoute;