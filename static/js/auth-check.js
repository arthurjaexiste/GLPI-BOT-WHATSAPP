// Verificação global de autenticação e nivel de permissão (RBAC) no Painel Web
async function checkAuthAndRole() {
    try {
        const res = await fetch('/api/me');
        if (!res.ok) {
            if (res.status === 401 && window.location.pathname !== '/login') {
                window.location.href = '/login';
            }
            return;
        }
        const user = await res.json();
        
        // Atualiza a badge do usuário no Navbar
        const badgeName = document.getElementById('user-badge-name');
        const badgeRole = document.getElementById('user-badge-role');
        const badgeContainer = document.getElementById('user-badge-header');

        if (badgeContainer) badgeContainer.classList.remove('hidden');
        if (badgeName) badgeName.innerText = user.name || user.username;
        if (badgeRole) {
            if (user.role === 'admin') {
                badgeRole.innerText = '⭐ Admin';
                badgeRole.className = 'px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-amber-500/20 border border-amber-500/30 text-amber-300';
            } else {
                badgeRole.innerText = '👤 Operador';
                badgeRole.className = 'px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-emerald-500/20 border border-emerald-500/30 text-emerald-300';
            }
        }

        // Se o perfil for operador, remove os links de rotas restritas aos administradores
        if (user.role === 'operator') {
            const adminSelectors = ['a[href="/flow"]', 'a[href="/config"]', 'a[href="/messages"]', 'a[href="/logs"]'];
            adminSelectors.forEach(sel => {
                document.querySelectorAll(sel).forEach(el => el.remove());
            });

            // Se o operador tentar acessar via URL direta uma página restrita, redireciona para a home
            const currentPath = window.location.pathname;
            if (['/config', '/flow', '/messages', '/logs'].includes(currentPath)) {
                window.location.href = '/chats';
            }
        }
    } catch (e) {
        console.error("Erro ao verificar autenticação do usuário:", e);
    }
}

document.addEventListener('DOMContentLoaded', checkAuthAndRole);
