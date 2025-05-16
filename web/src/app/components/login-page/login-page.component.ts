import { Component, inject } from '@angular/core'; // Import inject
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { AuthService } from '../../services/auth.service'; // Import AuthService
import { Router } from '@angular/router'; // Import Router

@Component({
  selector: 'app-login-page',
  standalone: true,
  imports: [FormsModule, CommonModule],
  templateUrl: './login-page.component.html',
  styleUrl: './login-page.component.css'
})
export class LoginPageComponent {
  private authService = inject(AuthService); // Inject AuthService
  private router = inject(Router); // Inject Router

  public uuidInput: string = ''; // To store the UUID entered by the user
  public errorMessage: string | null = null;
  public isLoading: boolean = false; // To manage loading state

  /**
   * Handles the UUID login process.
   * Calls the AuthService's loginWithUuid method and handles the response.
   */
  loginUser(): void {
    if (!this.uuidInput) {
      this.errorMessage = 'UUID cannot be empty.';
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;

    this.authService.loginWithUuid(this.uuidInput).subscribe({
      next: (response: { token: string } | { error: string; message?: string }) => {
        this.isLoading = false;
        if ('token' in response) {
          // Successful login, authService handles token storage
          // and isAuthenticated state.
          // Navigate to a protected route, e.g., profile page.
          this.router.navigate(['/profile']);
        } else {
          // Handle error response from the service
          this.errorMessage = response.message || response.error || 'Login failed. Please check your UUID or try again later.';
        }
      },
      error: (err: any) => { // Catch any unexpected errors from the Observable
        this.isLoading = false;
        this.errorMessage = err.error?.message || err.error?.error || err.message || 'An unexpected error occurred during login.';
        console.error('Login subscription error:', err);
      }
    });
  }
}
