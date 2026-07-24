// Carrega as configurações atuais da API
// Função fetchConfig manipula a rotina correspondente na interface do painel
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

        // SMTP
        document.getElementById('smtp_enabled').checked = cfg.smtp_enabled || false;
        document.getElementById('smtp_host').value = cfg.smtp_host || '';
        document.getElementById('smtp_port').value = cfg.smtp_port || 587;
        document.getElementById('smtp_username').value = cfg.smtp_username || '';
        document.getElementById('smtp_password').value = cfg.smtp_password || '';
        document.getElementById('smtp_sender').value = cfg.smtp_sender || '';
        document.getElementById('smtp_receiver').value = cfg.smtp_receiver || '';

        // Horários e Dias de Trabalho
        document.getElementById('working_hours_enabled').checked = cfg.working_hours_enabled || false;
        document.getElementById('working_hours_start').value = cfg.working_hours_start || '08:00';
        document.getElementById('working_hours_end').value = cfg.working_hours_end || '18:00';
        document.getElementById('msg_ausencia').value = cfg.msg_ausencia || '';

        // Marca os checkboxes dos dias de trabalho
        const workingDays = cfg.working_days ? cfg.working_days.split(',') : ['1', '2', '3', '4', '5'];
        const checkboxes = document.getElementsByName('working_days_checkbox');
        checkboxes.forEach(cb => {
            cb.checked = workingDays.includes(cb.value);
        });
    } catch (err) {
        showToast('Erro ao obter as configurações.', '❌');
    }
}

// Salva as configurações via POST na API
// Função saveConfig manipula a rotina correspondente na interface do painel
async function saveConfig() {
    const company_name = document.getElementById('company_name').value.trim();
    const telefone_notificacao = document.getElementById('telefone_notificacao').value.trim();
    const glpi_api_url = document.getElementById('glpi_api_url').value.trim();
    const glpi_app_token = document.getElementById('glpi_app_token').value.trim();
    const glpi_user_token = document.getElementById('glpi_user_token').value.trim();
    const dark_list = document.getElementById('dark_list').value.trim();
    const support_agents = document.getElementById('support_agents').value.trim();

    // SMTP
    const smtp_enabled = document.getElementById('smtp_enabled').checked;
    const smtp_host = document.getElementById('smtp_host').value.trim();
    const smtp_port = parseInt(document.getElementById('smtp_port').value) || 587;
    const smtp_username = document.getElementById('smtp_username').value.trim();
    const smtp_password = document.getElementById('smtp_password').value.trim();
    const smtp_sender = document.getElementById('smtp_sender').value.trim();
    const smtp_receiver = document.getElementById('smtp_receiver').value.trim();

    // Horários e Dias de Trabalho
    const working_hours_enabled = document.getElementById('working_hours_enabled').checked;
    const working_hours_start = document.getElementById('working_hours_start').value.trim();
    const working_hours_end = document.getElementById('working_hours_end').value.trim();
    const msg_ausencia = document.getElementById('msg_ausencia').value.trim();

    // Dias de Trabalho selecionados
    const selectedDays = [];
    const checkboxes = document.getElementsByName('working_days_checkbox');
    checkboxes.forEach(cb => {
        if (cb.checked) {
            selectedDays.push(cb.value);
        }
    });
    const working_days = selectedDays.join(',');

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
        support_agents,
        smtp_enabled,
        smtp_host,
        smtp_port,
        smtp_username,
        smtp_password,
        smtp_sender,
        smtp_receiver,
        working_hours_enabled,
        working_hours_start,
        working_hours_end,
        working_days,
        msg_ausencia
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

// Função toggleVisibility manipula a rotina correspondente na interface do painel
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
// Função copyToClipboard manipula a rotina correspondente na interface do painel
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

// Função showToast manipula a rotina correspondente na interface do painel
function showToast(message, icon = '✅') {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    toastIcon.innerText = icon;
    toastMsg.innerText = message;

    // Reset layout classes to avoid duplicate accumulation
    const baseClasses = "fixed bottom-6 right-6 px-5 py-3 rounded-lg shadow-2xl backdrop-blur-md transform transition duration-300 flex items-center gap-3 z-50";

    if (icon === '✅') {
        toast.className = baseClasses + " bg-emerald-950/80 border border-emerald-500/30 text-emerald-300";
    } else {
        toast.className = baseClasses + " bg-red-950/80 border border-red-500/30 text-red-300";
    }

    // Force transition by removing classes in a new microtask
    setTimeout(() => {
        toast.classList.remove('translate-y-24', 'opacity-0');
    }, 10);

    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

// Altera a senha do administrador
// Função changePassword manipula a rotina correspondente na interface do painel
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
// Função restartBot manipula a rotina correspondente na interface do painel
async function restartBot() {
    if (!confirm('Tem certeza de que deseja reiniciar o bot? O painel ficará temporariamente indisponível por alguns segundos.')) {
        return;
    }

    try {
        const response = await fetch('/api/restart', {
            method: 'POST'
        });

        if (!response.ok) throw new Error('Falha ao enviar sinal de reinício');

        showToast('Sinal de reinício enviado! Aguardando o bot voltar...', '✅');

        // Bloqueia a tela informando que está reiniciando
        setTimeout(() => {
            document.body.innerHTML = `
                <div class="min-h-screen flex items-center justify-center bg-[#0b0f19] text-white flex-col gap-4">
                    <span class="text-5xl animate-spin text-indigo-500">🔄</span>
                    <h2 class="text-xl font-bold animate-pulse">Reiniciando o Sistema...</h2>
                    <p class="text-xs text-gray-400" id="restart-msg">Aguardando o bot subir novamente...</p>
                </div>
            `;

            // Inicia tentativa de reconexão automática após 4 segundos
            setTimeout(autoReconnect, 4000);
        }, 1000);
    } catch (err) {
        showToast('Erro ao reiniciar o bot.', '❌');
    }
}

// Tenta se reconectar ao bot em loop até o painel voltar, então redireciona para a home
// Função autoReconnect manipula a rotina correspondente na interface do painel
async function autoReconnect() {
    const msgEl = document.getElementById('restart-msg');
    try {
        // Tenta buscar o status da API
        const res = await fetch('/api/status');
        if (res.ok) {
            if (msgEl) msgEl.innerText = "Conectado! Redirecionando...";
            setTimeout(() => {
                window.location.href = '/';
            }, 1000);
            return;
        }
    } catch (err) {
        // Ignora erros de rede enquanto o servidor estiver fora do ar
        console.log("Servidor ainda offline, tentando novamente...");
    }

    // Tenta novamente após 1.5 segundos
    setTimeout(autoReconnect, 1500);
}

// Inicializa buscando as configurações
window.addEventListener('DOMContentLoaded', fetchConfig);

// Testar conexão SMTP antes de salvar
// Função testSMTP manipula a rotina correspondente na interface do painel
async function testSMTP() {
    const company_name = document.getElementById('company_name').value.trim();
    const smtp_host = document.getElementById('smtp_host').value.trim();
    const smtp_port = parseInt(document.getElementById('smtp_port').value) || 587;
    const smtp_username = document.getElementById('smtp_username').value.trim();
    const smtp_password = document.getElementById('smtp_password').value;
    const smtp_sender = document.getElementById('smtp_sender').value.trim();
    const smtp_receiver = document.getElementById('smtp_receiver').value.trim();

    if (!smtp_host || !smtp_username || !smtp_password || !smtp_sender || !smtp_receiver) {
        showToast('Preencha todos os campos do SMTP antes de testar.', '⚠️');
        return;
    }

    const btn = document.getElementById('btn-test-smtp');
    if (!btn) return;

    const originalText = btn.innerText;
    btn.disabled = true;
    btn.innerText = "⏳ Testando...";

    const payload = {
        company_name,
        smtp_host,
        smtp_port,
        smtp_username,
        smtp_password,
        smtp_sender,
        smtp_receiver
    };

    try {
        const response = await fetch('/api/config/test-smtp', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        let data;
        try {
            data = await response.json();
        } catch (e) {
            data = { status: 'error', message: 'Servidor retornou resposta inválida' };
        }

        btn.disabled = false;
        btn.innerText = originalText;

        if (response.ok && data.status === 'ok') {
            showToast('E-mail de teste enviado com sucesso! Verifique a caixa de entrada.', '✅');
        } else {
            showToast('Falha no envio: ' + (data.message || 'Erro desconhecido'), '❌');
        }
    } catch (err) {
        btn.disabled = false;
        btn.innerText = originalText;
        console.error(err);
        showToast('Erro de conexão ao testar o SMTP.', '❌');
    }
}
