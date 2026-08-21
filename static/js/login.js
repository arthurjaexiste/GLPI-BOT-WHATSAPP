const canvas = document.getElementById('interactive-canvas');
const ctx = canvas.getContext('2d');
const spotlight = document.getElementById('glow-spotlight');

let width = canvas.width = window.innerWidth;
let height = canvas.height = window.innerHeight;

let particles = [];
let mouse = { x: null, y: null, active: false };

// Handle Resize
window.addEventListener('resize', () => {
    width = canvas.width = window.innerWidth;
    height = canvas.height = window.innerHeight;
    initParticles();
});

// Track Mouse
window.addEventListener('mousemove', (e) => {
    mouse.x = e.clientX;
    mouse.y = e.clientY;
    mouse.active = true;

    // Update smooth ambient spotlight position
    if (spotlight) {
        spotlight.style.left = `${mouse.x - 225}px`;
        spotlight.style.top = `${mouse.y - 225}px`;
    }
});

window.addEventListener('mouseleave', () => {
    mouse.active = false;
});

// Track Click Explosion
window.addEventListener('click', (e) => {
    // Burst of sparks from click location
    const burstCount = 20;
    for (let i = 0; i < burstCount; i++) {
        particles.push(new Particle(e.clientX, e.clientY, true, false));
    }
});

// Particle Class
class Particle {
    constructor(x, y, isSpark = false, isTrail = false) {
        this.x = x !== undefined ? x : Math.random() * width;
        this.y = y !== undefined ? y : Math.random() * height;
        this.isSpark = isSpark;
        this.isTrail = isTrail;
        this.radius = isSpark ? Math.random() * 2 + 1.2 : (isTrail ? Math.random() * 1.5 + 0.8 : Math.random() * 1.5 + 0.8);

        if (isSpark) {
            const angle = Math.random() * Math.PI * 2;
            const speed = Math.random() * 4 + 1.5;
            this.vx = Math.cos(angle) * speed;
            this.vy = Math.sin(angle) * speed;
            this.life = 1.0;
            this.decay = Math.random() * 0.02 + 0.015;
        } else if (isTrail) {
            this.vx = (Math.random() - 0.5) * 0.4;
            this.vy = (Math.random() - 0.5) * 0.4;
            this.life = 1.0;
            this.decay = Math.random() * 0.03 + 0.02;
        } else {
            this.baseVx = (Math.random() - 0.5) * 0.4;
            this.baseVy = (Math.random() - 0.5) * 0.4;
            this.vx = this.baseVx;
            this.vy = this.baseVy;
            this.life = 1.0;
            this.decay = 0;
        }
    }

    update() {
        if (this.isSpark || this.isTrail) {
            this.x += this.vx;
            this.y += this.vy;
            this.vx *= 0.97; // Friction
            this.vy *= 0.97;
            this.life -= this.decay;
        } else {
            // Interactive physics - antigravity repulsion
            if (mouse.active && mouse.x !== null) {
                const dx = mouse.x - this.x;
                const dy = mouse.y - this.y;
                const dist = Math.hypot(dx, dy);
                const limit = 160;

                if (dist < limit) {
                    // Repel from mouse to create an antigravity bubble effect
                    const force = (limit - dist) / limit;
                    const angle = Math.atan2(dy, dx);
                    this.vx -= Math.cos(angle) * force * 0.5;
                    this.vy -= Math.sin(angle) * force * 0.5;
                }
            }

            // Add simple damping and return to base speed
            this.vx += (this.baseVx - this.vx) * 0.05;
            this.vy += (this.baseVy - this.vy) * 0.05;

            this.x += this.vx;
            this.y += this.vy;

            // Screen Boundary Wrap-around
            if (this.x < 0) this.x = width;
            if (this.x > width) this.x = 0;
            if (this.y < 0) this.y = height;
            if (this.y > height) this.y = 0;
        }
    }

    draw() {
        ctx.save();
        if (this.isSpark) {
            ctx.shadowBlur = 8;
            ctx.shadowColor = '#ffffff';
            ctx.fillStyle = `rgba(255, 255, 255, ${this.life})`;
        } else if (this.isTrail) {
            ctx.shadowBlur = 4;
            ctx.shadowColor = '#a1a1aa';
            ctx.fillStyle = `rgba(161, 161, 170, ${this.life * 0.5})`;
        } else {
            ctx.fillStyle = 'rgba(255, 255, 255, 0.4)';
        }
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.radius, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
    }
}

function initParticles() {
    particles = [];
    // Base background wandering particles
    const count = Math.min(Math.floor((width * height) / 16000), 90);
    for (let i = 0; i < count; i++) {
        particles.push(new Particle());
    }
}

function drawLines() {
    // Draw lines only between background wandering particles to avoid clutter
    const bgParticles = particles.filter(p => !p.isSpark && !p.isTrail);
    for (let i = 0; i < bgParticles.length; i++) {
        for (let j = i + 1; j < bgParticles.length; j++) {
            const dx = bgParticles[i].x - bgParticles[j].x;
            const dy = bgParticles[i].y - bgParticles[j].y;
            const dist = Math.hypot(dx, dy);
            const limit = 110;

            if (dist < limit) {
                const alpha = (1 - (dist / limit)) * 0.06;
                ctx.strokeStyle = `rgba(255, 255, 255, ${alpha})`;
                ctx.lineWidth = 0.6;
                ctx.beginPath();
                ctx.moveTo(bgParticles[i].x, bgParticles[i].y);
                ctx.lineTo(bgParticles[j].x, bgParticles[j].y);
                ctx.stroke();
            }
        }
    }
}

function animate() {
    ctx.clearRect(0, 0, width, height);

    // Filter out dead particles
    particles = particles.filter(p => !((p.isSpark || p.isTrail) && p.life <= 0));

    particles.forEach(p => {
        p.update();
        p.draw();
    });

    drawLines();

    requestAnimationFrame(animate);
}

initParticles();
animate();
