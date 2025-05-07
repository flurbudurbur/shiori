import { Component } from '@angular/core';
import { BookmarkButtonComponent } from '../bookmark-button/bookmark-button.component';

@Component({
  selector: 'app-hero',
  imports: [BookmarkButtonComponent],
  templateUrl: './hero.component.html',
  styleUrl: './hero.component.css',
})
export class HeroComponent {
  public handleHeroButtonClick(): void {
    // console.log('Hero button clicked!');
  }
}
