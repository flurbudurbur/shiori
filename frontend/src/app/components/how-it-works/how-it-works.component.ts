import { Component, OnInit } from '@angular/core';
import { TitleService } from '../../services/title.service';
import { effect } from '@angular/core';

@Component({
  selector: 'app-how-it-works',
  standalone: true,
  imports: [],
  templateUrl: './how-it-works.component.html',
  styleUrl: './how-it-works.component.css'
})
export class HowItWorksComponent implements OnInit {
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
