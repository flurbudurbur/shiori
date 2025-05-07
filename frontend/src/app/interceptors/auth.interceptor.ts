import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from '../services/auth.service'; // Import AuthService

/**
 * Auth interceptor function to add Bearer token and handle authentication errors
 */
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const authService = inject(AuthService);
  const router = inject(Router);
  const token = authService.getToken();

  let authReq = req;
  if (token) {
    authReq = req.clone({
      setHeaders: {
        Authorization: `Bearer ${token}`
      }
    });
  }

  return next(authReq).pipe(
    catchError((error: HttpErrorResponse) => {
      // Handle authentication errors
      if (error.status === 401) {
        console.error('Unauthorized request:', error);
        // Clear token and redirect to login page
        // authService.logout(); // Consider if logout action (clearing token, etc.) should be here or handled by component
        router.navigate(['/login']);
      }
      return throwError(() => error);
    })
  );
};