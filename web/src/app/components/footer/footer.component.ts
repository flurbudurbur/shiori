import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TitleService } from '../../services/title.service';
import { effect } from '@angular/core';

@Component({
  selector: 'app-footer',
  imports: [RouterLink],
  templateUrl: './footer.component.html',
  styleUrl: './footer.component.css'
})
export class FooterComponent {
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
}
