import { Component, OnInit } from '@angular/core';
import { ApiService } from '../../services/api.service';
import { AuthService } from '../../services/auth.service'; // Import AuthService
import { Router } from '@angular/router'; // Import Router

@Component({
  selector: 'app-bookmark-button',
  // imports: [], // No changes needed here for standalone components regarding services/HttpClientModule
  templateUrl: './bookmark-button.component.html',
  styleUrl: './bookmark-button.component.css',
})
export class BookmarkButtonComponent implements OnInit {
  constructor(
    private apiService: ApiService,
    private authService: AuthService, // Inject AuthService
    private router: Router // Inject Router
  ) {}

  ngOnInit(): void {
    // Initialize component
  }

  public getBookmark(): void {
    // Call the assumed backend endpoint
    // Use POST and correct the path (ApiService likely adds /api prefix)
    this.apiService
      .post<{ uuid: string }>('/auth/generate-bookmark', {})
      .subscribe({
        next: (response) => {
          if (response && response.uuid) {
            // Store UUID and set auth state using AuthService
            this.authService.loginWithUuid(response.uuid);
            // Navigate to the profile page
            this.router.navigate(['/profile']);
          } else {
            console.error(
              'Error: Invalid response format from generate-bookmark'
            );
            // Handle invalid response
          }
        },
        error: (error: any) => {
          console.error('Error getting bookmark UUID:', error);
          // Handle error (e.g., show error message to user)
        },
      });
  }
}
