#!/bin/bash
# Generate icons for SpamChecker
# This script creates favicon and app icons from an SVG source

cat > icon.svg << 'EOF'
<svg width="512" height="512" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
  <rect width="512" height="512" rx="100" fill="#0a0e1a"/>
  <circle cx="256" cy="256" r="180" stroke="#90caf9" stroke-width="20" fill="none"/>
  <path d="M256 150 L330 220 L300 320 L212 320 L182 220 Z" fill="#90caf9"/>
  <circle cx="256" cy="256" r="60" fill="#0a0e1a"/>
  <path d="M200 380 Q256 420 312 380" stroke="#f48fb1" stroke-width="15" fill="none" stroke-linecap="round"/>
</svg>
EOF

# Install required tools (if not installed)
# sudo apt-get install imagemagick

# Generate favicon.ico (multiple sizes)
convert icon.svg -resize 16x16 favicon-16.png
convert icon.svg -resize 32x32 favicon-32.png
convert icon.svg -resize 48x48 favicon-48.png
convert favicon-16.png favicon-32.png favicon-48.png frontend/public/favicon.ico

# Generate PNG icons
convert icon.svg -resize 192x192 frontend/public/logo192.png
convert icon.svg -resize 512x512 frontend/public/logo512.png

# Clean up temporary files
rm favicon-*.png icon.svg

echo "Icons generated successfully!"