import { Component, OnInit, OnDestroy } from '@angular/core';
import { NgClass } from '@angular/common'; // Import NgClass
import { RouterLink, RouterLinkActive } from '@angular/router';
import { TitleService } from '../../services/title.service';
import { effect } from '@angular/core';
import { BookmarkButtonComponent } from '../bookmark-button/bookmark-button.component';
import { LoginButtonComponent } from '../login-button/login-button.component'; // Import the new component

@Component({
  selector: 'app-nav',
  standalone: true,
  imports: [RouterLink, RouterLinkActive, BookmarkButtonComponent, LoginButtonComponent, NgClass], // Add LoginButtonComponent
  templateUrl: './nav.component.html',
  styleUrl: './nav.component.css'
})
export class NavComponent implements OnInit {
  title: string;
  isMobileMenuOpen = false; // Add property to track mobile menu state

  constructor(private titleService: TitleService) {
    // Get initial title
    this.title = this.titleService.getTitle();
    
    // Create an effect to update the title whenever it changes in the service
    effect(() => {
      this.title = this.titleService.getTitle();
    });
  }
  
  ngOnInit(): void {
    // console.log('NavComponent initialized with title:', this.title);
  }

  // Method to toggle the mobile menu
  toggleMobileMenu(): void {
    this.isMobileMenuOpen = !this.isMobileMenuOpen;
  }
}
