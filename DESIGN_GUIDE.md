# Neumorphic Design System - Quick Reference

This is a quick reference guide for using the Knitnox neumorphic design system.

## 🎨 Getting Started

Include the neumorphic CSS library in your HTML:

```html
<link rel="stylesheet" href="path/to/assets/css/neumorphic.css">
```

## 📦 Component Gallery

### Cards

```html
<!-- Basic Card -->
<div class="neumorph-card">
  <h3>Card Title</h3>
  <p>Card content goes here</p>
</div>

<!-- Clickable Card with Hover -->
<div class="neumorph-card-hover">
  <h3>Clickable Card</h3>
  <p>This card has hover effects</p>
</div>

<!-- Large Card -->
<div class="neumorph-card-large">
  <h1>Featured Content</h1>
</div>

<!-- Small Card -->
<div class="neumorph-card-small">
  <p>Compact content</p>
</div>
```

### Buttons

```html
<!-- Standard Button -->
<button class="neumorph-btn">Click Me</button>

<!-- Button Sizes -->
<button class="neumorph-btn neumorph-btn-sm">Small</button>
<button class="neumorph-btn">Regular</button>
<button class="neumorph-btn neumorph-btn-lg">Large</button>

<!-- Accent Button -->
<button class="neumorph-btn neumorph-btn-accent">Primary Action</button>

<!-- Button as Link -->
<a href="#" class="neumorph-btn">Link Button</a>
```

### Form Elements

```html
<!-- Text Input -->
<input type="text" class="neumorph-input" placeholder="Enter text...">

<!-- Textarea -->
<textarea class="neumorph-textarea" placeholder="Enter longer text..."></textarea>

<!-- Select Dropdown -->
<select class="neumorph-select">
  <option>Option 1</option>
  <option>Option 2</option>
  <option>Option 3</option>
</select>
```

### Layout Components

```html
<!-- Container -->
<div class="neumorph-container">
  <!-- Your content here -->
</div>

<!-- Flex Layout -->
<div class="neumorph-flex">
  <div>Item 1</div>
  <div>Item 2</div>
</div>

<!-- Centered Flex -->
<div class="neumorph-flex-center">
  <div>Centered Item 1</div>
  <div>Centered Item 2</div>
</div>

<!-- Space Between Flex -->
<div class="neumorph-flex-between">
  <div>Left Item</div>
  <div>Right Item</div>
</div>

<!-- Responsive Grid (2 cols mobile, 4 cols desktop) -->
<div class="neumorph-grid-responsive">
  <div class="neumorph-card">Item 1</div>
  <div class="neumorph-card">Item 2</div>
  <div class="neumorph-card">Item 3</div>
  <div class="neumorph-card">Item 4</div>
</div>

<!-- Fixed Grids -->
<div class="neumorph-grid-2">...</div>  <!-- 2 columns -->
<div class="neumorph-grid-3">...</div>  <!-- 3 columns -->
<div class="neumorph-grid-4">...</div>  <!-- 4 columns -->
```

### Typography

```html
<!-- Headings -->
<h1 class="neumorph-heading neumorph-heading-xl">Extra Large</h1>
<h2 class="neumorph-heading neumorph-heading-lg">Large</h2>
<h3 class="neumorph-heading neumorph-heading-md">Medium</h3>
<h4 class="neumorph-heading neumorph-heading-sm">Small</h4>

<!-- Text -->
<p class="neumorph-text">Regular paragraph text</p>
<p class="neumorph-text-small">Small text</p>
```

### Icons & Images

```html
<!-- Icon Container -->
<img src="icon.svg" class="neumorph-icon neumorph-icon-md" alt="Icon">

<!-- Icon Sizes -->
<img src="icon.svg" class="neumorph-icon neumorph-icon-sm" alt="Small">
<img src="icon.svg" class="neumorph-icon neumorph-icon-md" alt="Medium">
<img src="icon.svg" class="neumorph-icon neumorph-icon-lg" alt="Large">
```

### Special Components

```html
<!-- Loading Spinner -->
<div class="neumorph-spinner"></div>

<!-- Badge -->
<span class="neumorph-badge">New</span>

<!-- Divider -->
<hr class="neumorph-divider">

<!-- Progress Bar -->
<div class="neumorph-progress">
  <div class="neumorph-progress-bar" style="width: 60%;"></div>
</div>
```

## 🎯 Utility Classes

### Margin

```html
<div class="neumorph-mt-sm">...</div>  <!-- margin-top: small -->
<div class="neumorph-mt-md">...</div>  <!-- margin-top: medium -->
<div class="neumorph-mt-lg">...</div>  <!-- margin-top: large -->
<div class="neumorph-mt-xl">...</div>  <!-- margin-top: extra large -->
<div class="neumorph-mt-2xl">...</div> <!-- margin-top: 2x extra large -->

<!-- Same for margin-bottom: mb-sm, mb-md, mb-lg, mb-xl, mb-2xl -->
```

### Padding

```html
<div class="neumorph-p-sm">...</div>   <!-- padding: small -->
<div class="neumorph-p-md">...</div>   <!-- padding: medium -->
<div class="neumorph-p-lg">...</div>   <!-- padding: large -->
<div class="neumorph-p-xl">...</div>   <!-- padding: extra large -->
<div class="neumorph-p-2xl">...</div>  <!-- padding: 2x extra large -->
```

### Text Alignment

```html
<div class="neumorph-text-center">...</div>
<div class="neumorph-text-left">...</div>
<div class="neumorph-text-right">...</div>
```

### Display

```html
<div class="neumorph-block">...</div>
<div class="neumorph-inline-block">...</div>
<div class="neumorph-hidden">...</div>
```

### Width

```html
<div class="neumorph-w-full">...</div>  <!-- width: 100% -->
<div class="neumorph-w-half">...</div>  <!-- width: 50% -->
```

### Animations

```html
<div class="neumorph-fade-in">...</div>   <!-- Fade in animation -->
<div class="neumorph-slide-in">...</div>  <!-- Slide in animation -->
```

## 🎨 CSS Variables

Customize the design system by overriding these variables:

```css
:root {
  /* Colors */
  --neumorph-bg: #e6e9ef;
  --neumorph-light: #ffffff;
  --neumorph-dark: #c2c8d0;
  --neumorph-text: #2a2f3a;
  
  /* Accent Colors */
  --neumorph-accent-blue: #5a8dee;
  --neumorph-accent-red: #ff6b6b;
  --neumorph-accent-green: #51cf66;
  --neumorph-accent-orange: #ffa500;
  
  /* Border Radius */
  --neumorph-radius-sm: 8px;
  --neumorph-radius-md: 12px;
  --neumorph-radius-lg: 16px;
  --neumorph-radius-xl: 20px;
  
  /* Spacing */
  --neumorph-spacing-xs: 4px;
  --neumorph-spacing-sm: 8px;
  --neumorph-spacing-md: 12px;
  --neumorph-spacing-lg: 16px;
  --neumorph-spacing-xl: 20px;
  --neumorph-spacing-2xl: 24px;
  
  /* Transitions */
  --neumorph-transition-fast: 0.15s ease;
  --neumorph-transition-normal: 0.2s ease;
  --neumorph-transition-slow: 0.3s ease;
}
```

## 📝 Complete Example

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>My Neumorphic App</title>
  <link rel="stylesheet" href="assets/css/neumorphic.css">
  <style>
    body {
      background: var(--neumorph-bg);
      padding: 20px;
    }
  </style>
</head>
<body>
  <div class="neumorph-container">
    <!-- Header -->
    <div class="neumorph-card-large neumorph-text-center">
      <h1 class="neumorph-heading neumorph-heading-xl">My App</h1>
      <p class="neumorph-text">Welcome to my neumorphic app!</p>
    </div>
    
    <!-- Form -->
    <div class="neumorph-card neumorph-mt-lg">
      <h2 class="neumorph-heading neumorph-heading-md">Get Started</h2>
      <div class="neumorph-mt-md">
        <input type="text" class="neumorph-input" placeholder="Enter your name">
      </div>
      <div class="neumorph-mt-md">
        <select class="neumorph-select">
          <option>Select an option</option>
          <option>Option 1</option>
          <option>Option 2</option>
        </select>
      </div>
      <div class="neumorph-mt-md">
        <button class="neumorph-btn neumorph-btn-accent neumorph-w-full">
          Submit
        </button>
      </div>
    </div>
    
    <!-- Grid of Cards -->
    <div class="neumorph-grid-responsive neumorph-mt-lg">
      <div class="neumorph-card-hover neumorph-text-center">
        <img src="icon1.svg" class="neumorph-icon neumorph-icon-md" alt="Icon 1">
        <h3 class="neumorph-heading neumorph-heading-sm neumorph-mt-sm">
          Feature 1
        </h3>
        <p class="neumorph-text-small">Description of feature 1</p>
      </div>
      
      <div class="neumorph-card-hover neumorph-text-center">
        <img src="icon2.svg" class="neumorph-icon neumorph-icon-md" alt="Icon 2">
        <h3 class="neumorph-heading neumorph-heading-sm neumorph-mt-sm">
          Feature 2
        </h3>
        <p class="neumorph-text-small">Description of feature 2</p>
      </div>
      
      <div class="neumorph-card-hover neumorph-text-center">
        <img src="icon3.svg" class="neumorph-icon neumorph-icon-md" alt="Icon 3">
        <h3 class="neumorph-heading neumorph-heading-sm neumorph-mt-sm">
          Feature 3
        </h3>
        <p class="neumorph-text-small">Description of feature 3</p>
      </div>
      
      <div class="neumorph-card-hover neumorph-text-center">
        <img src="icon4.svg" class="neumorph-icon neumorph-icon-md" alt="Icon 4">
        <h3 class="neumorph-heading neumorph-heading-sm neumorph-mt-sm">
          Feature 4
        </h3>
        <p class="neumorph-text-small">Description of feature 4</p>
      </div>
    </div>
  </div>
</body>
</html>
```

## 🔗 Additional Resources

- See `assets/css/neumorphic.css` for the complete implementation
- Check `apps/calculator/` and `apps/color-calculator/` for real-world examples
- Read the main README.md for comprehensive documentation

---

**Happy Building! 🎨**

