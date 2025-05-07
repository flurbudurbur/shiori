import { Component, OnInit } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { FooterComponent } from './components/footer/footer.component';
import { NavComponent } from './components/nav/nav.component';
import { TitleService } from './services/title.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    RouterOutlet,
    NavComponent,
    FooterComponent
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent implements OnInit {
  // Local reference to title for use in this component if needed
  private appTitle = 'Bookmark';
  
  constructor(private titleService: TitleService) {}
  
  ngOnInit(): void {
    // Set the application title when the app initializes
    // This will update the title in the service, which will propagate to NavComponent
    this.titleService.setTitle(this.appTitle);
  }
  
  // Method to update title if needed from this component
  updateTitle(newTitle: string): void {
    this.appTitle = newTitle;
    this.titleService.setTitle(newTitle);
  }
}
