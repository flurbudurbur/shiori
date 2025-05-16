import { Component } from '@angular/core';
import { RouterLink } from '@angular/router'; // Import RouterLink

@Component({
  selector: 'app-login-button',
  standalone: true, // Ensure it's standalone
  imports: [RouterLink], // Add RouterLink here
  templateUrl: './login-button.component.html',
  styleUrl: './login-button.component.css'
})
export class LoginButtonComponent {

}
