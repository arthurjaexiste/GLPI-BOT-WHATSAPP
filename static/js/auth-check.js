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
        
        const badgeName = document.getElementById('user-badge-name');
        const badgeRole = document.getElementById('user-badge-role');
        const badgeContainer = document.getElementById('user-badge-header');

        if (badgeContainer) badgeContainer.classList.remove('hidden');
        if (badgeName) badgeName.innerText = user.name || user.username;
        if (badgeRole) {
            if (user.role === 'admin') {
                badgeRole.innerText = 'Admin';
                badgeRole.className = 'px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-slate-800 text-slate-200 border border-slate-700';
            } else {
                badgeRole.innerText = 'Operador';
                badgeRole.className = 'px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-slate-800 text-slate-300 border border-slate-700';
            }
        }

        if (user.role === 'operator') {
            const adminSelectors = ['a[href="/flow"]', 'a[href="/config"]', 'a[href="/messages"]', 'a[href="/logs"]'];
            adminSelectors.forEach(sel => {
                document.querySelectorAll(sel).forEach(el => el.remove());
            });

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
