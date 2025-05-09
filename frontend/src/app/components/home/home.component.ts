import { Component } from '@angular/core';
import { HeroComponent } from '../hero/hero.component';
import { FeaturesComponent } from '../features/features.component';
import { HowItWorksComponent } from '../how-it-works/how-it-works.component';
import { PrivacyCommitmentComponent } from '../privacy-commitment/privacy-commitment.component';
import { FinalCtaComponent } from '../final-cta/final-cta.component';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [
    HeroComponent,
    FeaturesComponent,
    HowItWorksComponent,
    PrivacyCommitmentComponent,
    FinalCtaComponent
  ],
  templateUrl: './home.component.html',
  styleUrl: './home.component.css'
})
export class HomeComponent {
  // Home component specific logic can be added here
}