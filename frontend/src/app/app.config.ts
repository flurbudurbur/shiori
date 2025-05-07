import { ApplicationConfig, provideZoneChangeDetection, APP_INITIALIZER } from '@angular/core';
import { provideRouter, Routes } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { authInterceptor } from './interceptors/auth.interceptor';
import { AboutComponent } from './components/about/about.component';
import { ContactComponent } from './components/contact/contact.component';
import { HomeComponent } from './components/home/home.component';
import { ProfileComponent } from './components/profile/profile.component'; // Import ProfileComponent
import { LoginPageComponent } from './components/login-page/login-page.component'; // Import LoginPageComponent
import { TitleService } from './services/title.service';

const routes: Routes = [
  // Default route redirects to home
  { path: '', redirectTo: 'home', pathMatch: 'full' },
  // Explicit home route using the dedicated HomeComponent
  { path: 'home', component: HomeComponent },
  { path: 'about', component: AboutComponent },
  { path: 'contact', component: ContactComponent },
  { path: 'profile', component: ProfileComponent }, // Add profile route
  { path: 'login', component: LoginPageComponent }, // Add login route
  // Wildcard route for 404 handling - redirects to home for now
  { path: '**', redirectTo: 'home' }
];

// Function to initialize the TitleService
function initializeTitleService(titleService: TitleService) {
  return () => {
    // You could set a default title or load from storage/API here
    return titleService.getTitle(); // Just to trigger the service initialization
  };
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
    // Add APP_INITIALIZER to ensure TitleService is initialized early
    {
      provide: APP_INITIALIZER,
      useFactory: initializeTitleService,
      deps: [TitleService],
      multi: true
    }
  ]
};
