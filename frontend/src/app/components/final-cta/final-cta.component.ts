import { Component } from '@angular/core';
import { BookmarkButtonComponent } from '../bookmark-button/bookmark-button.component'; // Import the new component

@Component({
  selector: 'app-final-cta',
  imports: [BookmarkButtonComponent],
  templateUrl: './final-cta.component.html',
  styleUrl: './final-cta.component.css'
})
export class FinalCtaComponent {
}
