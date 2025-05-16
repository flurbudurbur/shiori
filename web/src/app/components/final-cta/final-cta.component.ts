import { Component, OnInit } from '@angular/core';
import { BookmarkButtonComponent } from '../bookmark-button/bookmark-button.component'; // Import the new component
import { TitleService } from '../../services/title.service';
import { effect } from '@angular/core';

@Component({
  selector: 'app-final-cta',
  standalone: true,
  imports: [BookmarkButtonComponent],
  templateUrl: './final-cta.component.html',
  styleUrl: './final-cta.component.css'
})
export class FinalCtaComponent implements OnInit {
  appTitle: string;

  constructor(private titleService: TitleService) {
    // Get initial title
    this.appTitle = this.titleService.getTitle();
    
    // Create an effect to update the title whenever it changes in the service
    effect(() => {
      this.appTitle = this.titleService.getTitle();
    });
  }

  ngOnInit(): void {
    // Initialize component
  }
}
