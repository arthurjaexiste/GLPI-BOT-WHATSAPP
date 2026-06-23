// Carrega as configurações atuais da API
async function fetchConfig() {
    try {
        const response = await fetch('/api/config');
        if (!response.ok) throw new Error('Falha ao obter configurações');
        const cfg = await response.json();
        
        document.getElementById('company_name').value = cfg.company_name || '';
        document.getElementById('telefone_notificacao').value = cfg.telefone_notificacao || '';
        document.getElementById('glpi_api_url').value = cfg.glpi_api_url || '';
        document.getElementById('glpi_app_token').value = cfg.glpi_app_token || '';
        document.getElementById('glpi_user_token').value = cfg.glpi_user_token || '';
        document.getElementById('dark_list').value = cfg.dark_list || '';
        document.getElementById('support_agents').value = cfg.support_agents || '';
    } catch (err) {
        showToast('Erro ao obter as configurações.', '❌');
    }
}

// Salva as configurações via POST na API
async function saveConfig() {
    const company_name = document.getElementById('company_name').value.trim();
    const telefone_notificacao = document.getElementById('telefone_notificacao').value.trim();
    const glpi_api_url = document.getElementById('glpi_api_url').value.trim();
    const glpi_app_token = document.getElementById('glpi_app_token').value.trim();
    const glpi_user_token = document.getElementById('glpi_user_token').value.trim();
    const dark_list = document.getElementById('dark_list').value.trim();
    const support_agents = document.getElementById('support_agents').value.trim();

    if (!glpi_api_url) {
        showToast('O Link da API do GLPI é obrigatório.', '⚠️');
        return;
    }

    const payload = {
        company_name,
        telefone_notificacao,
        glpi_api_url,
        glpi_app_token,
        glpi_user_token,
        dark_list,
        support_agents
    };

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (!response.ok) throw new Error('Erro ao salvar as configurações');
        showToast('Configurações salvas e aplicadas com sucesso!', '✅');
    } catch (err) {
        showToast('Erro ao salvar configurações.', '❌');
    }
}

// Alterna entre mostrar e esconder a senha (tipo password / text)
function toggleVisibility(btn, id) {
    const input = document.getElementById(id);
    if (input.type === "password") {
        input.type = "text";
        btn.innerText = "🔒";
        btn.title = "Esconder Token";
    } else {
        input.type = "password";
        btn.innerText = "👁️";
        btn.title = "Visualizar Token";
    }
}

// Copia o valor do token para a área de transferência
async function copyToClipboard(id) {
    const input = document.getElementById(id);
    const value = input.value;
    if (!value) {
        showToast('Nada para copiar.', '⚠️');
        return;
    }
    try {
        await navigator.clipboard.writeText(value);
        showToast('Token copiado para a área de transferência!', '✅');
    } catch (err) {
        showToast('Erro ao copiar token.', '❌');
    }
}

// Exibe um toast temporário
function showToast(message, icon = '✅') {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    toastIcon.innerText = icon;
    toastMsg.innerText = message;

    if (icon === '✅') {
        toast.className = toast.className.replace('border-red-500/30', 'border-emerald-500/30')
                                       .replace('bg-red-950/80', 'bg-emerald-950/80')
                                       .replace('text-red-300', 'text-emerald-300') + ' border-emerald-500/30 bg-emerald-950/80 text-emerald-300';
    } else {
        toast.className = toast.className.replace('border-emerald-500/30', 'border-red-500/30')
                                       .replace('bg-emerald-950/80', 'bg-red-950/80')
                                       .replace('text-emerald-300', 'text-red-300') + ' border-red-500/30 bg-red-950/80 text-red-300';
    }

    toast.classList.remove('translate-y-24', 'opacity-0');
    
    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

// Altera a senha do administrador
async function changePassword(event) {
    event.preventDefault();
    const current_password = document.getElementById('current_password').value;
    const new_password = document.getElementById('new_password').value;
    const confirm_password = document.getElementById('confirm_password').value;

    if (new_password !== confirm_password) {
        showToast('A nova senha e a confirmação não coincidem!', '⚠️');
        return;
    }

    try {
        const response = await fetch('/api/change-password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ current_password, new_password })
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao alterar a senha');
        }

        showToast('Senha alterada com sucesso!', '✅');
        document.getElementById('change-password-form').reset();
    } catch (err) {
        showToast(err.message || 'Erro ao alterar a senha.', '❌');
    }
}

// Reinicia o bot
async function restartBot() {
    if (!confirm('Tem certeza de que deseja reiniciar o bot? O painel ficará temporariamente indisponível por alguns segundos.')) {
        return;
    }

    try {
        const response = await fetch('/api/restart', {
            method: 'POST'
        });

        if (!response.ok) throw new Error('Falha ao enviar sinal de reinício');

        showToast('Sinal de reinício enviado! Aguarde alguns segundos e atualize a página.', '✅');
        
        // Bloqueia a tela informando que está reiniciando
        setTimeout(() => {
            document.body.innerHTML = `
                <div class="min-h-screen flex items-center justify-center bg-[#0b0f19] text-white flex-col gap-4">
                    <span class="text-5xl animate-spin">🔄</span>
                    <h2 class="text-xl font-bold">Reiniciando o Sistema...</h2>
                    <p class="text-xs text-gray-400">Aguarde 5 segundos e tente recarregar a página.</p>
                    <button onclick="window.location.reload()" class="mt-4 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 rounded-lg text-xs font-bold transition">
                        Recarregar Página
                    </button>
                </div>
            `;
        }, 1000);
    } catch (err) {
        showToast('Erro ao reiniciar o bot.', '❌');
    }
}

// Inicializa buscando as configurações
window.addEventListener('DOMContentLoaded', fetchConfig);
