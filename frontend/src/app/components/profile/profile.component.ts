import { Component, OnInit, Signal, WritableSignal, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AuthService } from '../../services/auth.service';
import { ApiService } from '../../services/api.service'; // Import ApiService
import { FormsModule } from '@angular/forms'; // Import FormsModule if needed for other inputs, or remove if not

@Component({
  selector: 'app-profile',
  standalone: true,
  imports: [CommonModule, FormsModule], // Add FormsModule if you use ngModel or other forms features
  templateUrl: './profile.component.html',
  styleUrl: './profile.component.css'
})
export class ProfileComponent implements OnInit {
  public apiTokenSignal: Signal<string | null>;
  uuidSignal: Signal<string | null>; // Declare the property
  public newApiToken: WritableSignal<string | null> = signal(null);
  public showNewToken: WritableSignal<boolean> = signal(false);
  public resetTokenInProgress: WritableSignal<boolean> = signal(false);
  public errorMessage: WritableSignal<string | null> = signal(null);
  public copied: WritableSignal<boolean> = signal(false);
  public generatedUuid: WritableSignal<string | null> = signal(null); // Make it a signal for reactivity
  public uuidLoadingError: WritableSignal<string | null> = signal(null); // To display UUID loading errors

  constructor(private authService: AuthService, private apiService: ApiService) { // Inject ApiService
    this.apiTokenSignal = this.authService.apiTokenSignal;
    this.uuidSignal = this.authService.userUuidSignal; // Assign in constructor

    if (this.apiTokenSignal() === null) {
      console.warn('ProfileComponent: Initial API Token from signal is null.');
    }
  }

  ngOnInit(): void {
    this.fetchNewUuid();
  }

  fetchNewUuid(): void {
    this.uuidLoadingError.set(null);
    this.apiService.fetchUuid().subscribe({
      next: (response) => {
        if (response && response.uuid) {
          this.generatedUuid.set(response.uuid);
        } else {
          this.generatedUuid.set('Error: Invalid UUID response');
          this.uuidLoadingError.set('Failed to retrieve a valid UUID from the server.');
          console.error('Invalid UUID response:', response);
        }
      },
      error: (err) => {
        console.error('Error fetching UUID:', err);
        this.generatedUuid.set('Error: Could not load UUID');
        this.uuidLoadingError.set('An error occurred while fetching the UUID.');
      }
    });
  }

  resetToken(): void {
    this.resetTokenInProgress.set(true);
    this.errorMessage.set(null);
    this.newApiToken.set(null);
    this.showNewToken.set(false);
    this.copied.set(false);

    this.authService.resetApiToken().subscribe({
      next: (response: { token: string } | { error: string }) => {
        if (response && 'token' in response && typeof response.token === 'string') {
          this.newApiToken.set(response.token);
          this.showNewToken.set(true); // Show the new token section
          this.errorMessage.set(null); // Clear any previous error
        } else if (response && 'error' in response && typeof response.error === 'string') {
          console.error('Error resetting API token (from response object):', response.error);
          this.errorMessage.set(response.error);
          this.newApiToken.set(null); // Ensure new token is null on error
          this.showNewToken.set(false); // Ensure new token section is hidden
        } else {
          const unexpectedErrorMsg = 'Unexpected response structure while resetting token.';
          console.error(unexpectedErrorMsg, response);
          this.errorMessage.set(unexpectedErrorMsg);
          this.newApiToken.set(null);
          this.showNewToken.set(false);
        }
        this.resetTokenInProgress.set(false);
      },
      error: (err: any) => { // Added 'any' type for err for broader compatibility with error shapes
        console.error('Error resetting API token (observable error):', err);
        let errMsg = 'An unexpected error occurred while resetting the token.';
        // Try to extract a meaningful error message
        if (err && err.error && typeof err.error.error === 'string') {
          errMsg = err.error.error;
        } else if (err && err.error && typeof err.error === 'string') {
          errMsg = err.error;
        } else if (err && typeof err.message === 'string') {
          errMsg = err.message;
        }
        this.errorMessage.set(errMsg);
        this.newApiToken.set(null); // Clear token on error
        this.showNewToken.set(false); // Hide token section on error
        this.resetTokenInProgress.set(false);
      }
    });
  }

  copyToClipboard(token: string | null): void {
    if (token && navigator.clipboard) {
      navigator.clipboard.writeText(token).then(() => {
        this.copied.set(true);
        setTimeout(() => this.copied.set(false), 2000); // Hide "Copied!" message after 2 seconds
      }).catch(err => {
        console.error('Failed to copy token: ', err);
        this.errorMessage.set('Failed to copy token to clipboard.');
      });
    }
  }

  hideNewToken(): void {
    this.showNewToken.set(false);
    this.newApiToken.set(null); // Clear the token so it's not shown again if the section is re-opened
  }
}
