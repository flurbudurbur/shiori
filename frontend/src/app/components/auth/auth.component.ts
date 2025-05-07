import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-auth',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './auth.component.html',
  styleUrl: './auth.component.css'
})
export class AuthComponent implements OnInit {
  errorMessage = '';
  showApiKey: boolean = false;
  generatedApiKey: any = null;
  isGenerating: boolean = false;

  constructor(
    private authService: AuthService,
  ) {}

  ngOnInit(): void {
    // Check if user is authenticated
    this.authService.isAuthenticated$.subscribe(isAuthenticated => {
    });
  }

  // generateBookmark() {
  //   // Placeholder for actual generation logic
  // }
}
