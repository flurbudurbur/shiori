import { Injectable, signal } from '@angular/core'; // Added signal
import { ApiService } from './api.service';
import { BehaviorSubject, Observable, tap, catchError, of } from 'rxjs';
import { Router } from '@angular/router';

export interface User {
  username: string;
  password: string;
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private isAuthenticatedSubject = new BehaviorSubject<boolean>(false);
  public isAuthenticated$ = this.isAuthenticatedSubject.asObservable();
  // Use Angular Signal for reactive API Token
  private apiTokenSignalWritable = signal<string | null>(null);
  public apiTokenSignal = this.apiTokenSignalWritable.asReadonly(); // Expose as readonly signal
  private readonly TOKEN_KEY = 'api_token';
  public userUuidSignal = signal<string | null>(null); // Added userUuidSignal

  constructor(
    private apiService: ApiService,
    private router: Router
  ) {
    // Check if user is already authenticated on service initialization
    this.loadTokenFromStorage();
    this.checkAuthStatus();
  }

  private loadTokenFromStorage(): void {
    if (typeof localStorage !== 'undefined') {
      const token = localStorage.getItem(this.TOKEN_KEY);
      if (token) {
        this.apiTokenSignalWritable.set(token);
        this.isAuthenticatedSubject.next(true);
      }
    }
  }

  private saveTokenToStorage(token: string): void {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(this.TOKEN_KEY, token);
    }
  }

  private removeTokenFromStorage(): void {
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem(this.TOKEN_KEY);
    }
  }

  getToken(): string | null {
    return this.apiTokenSignalWritable();
  }

  /**
   * Login user
   */
  login(user: User): Observable<any> { // Assuming login also returns a token
    return this.apiService.post<any>('/auth/login', user).pipe(
      tap((response) => { // Assuming response contains { token: '...' }
        if (response && response.token) {
          this.loginWithToken(response.token);
        } else {
          this.isAuthenticatedSubject.next(false); // Ensure not authenticated if token is missing
        }
      }),
      catchError(error => {
        console.error('Login failed:', error);
        this.isAuthenticatedSubject.next(false);
        this.apiTokenSignalWritable.set(null);
        this.removeTokenFromStorage();
        return of({ error: 'Login failed' });
      })
    );
  }

  /**
   * Logout user
   */
  logout(): Observable<any> {
    // Optionally, call a backend logout endpoint if it exists and needs to invalidate the token server-side
    // For now, just clearing local state
    return this.apiService.post<any>('/auth/logout', {}).pipe(
        tap(() => {
            this.clearAuthData();
            this.router.navigate(['/home']);
        }),
        catchError(error => {
            console.error('Logout failed on server:', error);
            // Even if server logout fails, clear local data
            this.clearAuthData();
            this.router.navigate(['/home']);
            return of({ error: 'Logout failed but cleared locally' });
        })
    );
  }

  private clearAuthData(): void {
    this.isAuthenticatedSubject.next(false);
    this.apiTokenSignalWritable.set(null);
    this.removeTokenFromStorage();
  }


  /**
   * Check if user is authenticated by validating the stored token
   */
  checkAuthStatus(): void {
    const token = this.getToken();
    if (token) {
      // Assuming /auth/validate will check the token provided by AuthInterceptor
      this.apiService.get<any>('/auth/validate').pipe(
        tap(() => {
          this.isAuthenticatedSubject.next(true);
        }),
        catchError(() => {
          this.clearAuthData(); // Token is invalid or expired
          return of(null);
        })
      ).subscribe();
    } else {
      this.clearAuthData();
    }
  }

  /**
   * Register a new user.
   * Expects a plain token in response.
   */
  register(): Observable<{ token: string } | { error: string }> {
    return this.apiService.post<{ token: string }>('/auth/register', {}).pipe(
      tap((response) => {
        if (response && response.token) {
          this.loginWithToken(response.token);
        } else {
          console.error('Registration response missing token:', response);
          this.clearAuthData();
        }
      }),
      catchError(error => {
        console.error('Registration failed:', error);
        this.clearAuthData();
        return of({ error: 'Registration failed' });
      })
    );
  }

  /**
   * Check if onboarding is possible
   */
  canOnboard(): Observable<boolean> {
    // This endpoint might change or be removed if registration is always open
    // or handled differently with the new token system.
    // For now, keeping it as is.
    return this.apiService.get<any>('/auth/register/status').pipe(
      tap(() => true),
      catchError(() => of(false))
    );
  }

  /**
   * Store API Token and set authenticated state
   */
  loginWithToken(token: string): void {
    this.apiTokenSignalWritable.set(token);
    this.saveTokenToStorage(token);
    this.isAuthenticatedSubject.next(true);
  }

  /**
   * Reset API Token by calling the new backend endpoint.
   * Displays the new token to the user once.
   */
  resetApiToken(): Observable<{ token: string } | { error: string }> {
    return this.apiService.post<{ token: string }>('/profile/api-token', {}).pipe(
      tap((response) => {
        if (response && response.token) {
          this.loginWithToken(response.token); // Store the new token
          // The component calling this will be responsible for displaying the token
        } else {
          console.error('Reset token response missing token:', response);
          // Optionally handle this error more gracefully in the UI
        }
      }),
      catchError(error => {
        console.error('Failed to reset API token:', error);
        // Don't clear existing token on failure, user might still have a valid one
        return of({ error: 'Failed to reset API token' });
      })
    );
  }

  public loginWithUuid(uuid: string): Observable<{ token: string } | { error: string; message?: string }> {
    return this.apiService.post<{ token: string }>('/auth/login-uuid', { uuid }).pipe(
      tap((response) => {
        if (response && response.token) {
          this.loginWithToken(response.token);
          this.userUuidSignal.set(uuid); // Store the UUID used for login
        } else {
          console.error('Login with UUID response missing token:', response);
          this.clearAuthData(); // Ensure not authenticated if token is missing
        }
      }),
      catchError(error => {
        console.error('Login with UUID failed:', error);
        this.clearAuthData();
        // Ensure the error object passed to the component has a consistent structure
        const errorMessage = error.error?.message || error.error?.error || 'Login with UUID failed';
        return of({ error: errorMessage, message: errorMessage });
      })
    );
  }
}