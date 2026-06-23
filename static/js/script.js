let statusInterval;
let lastQR = "";

async function checkStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();
        
        const qrStatusText = document.getElementById('qr-status');
        const qrCodeDiv = document.getElementById('qrcode');
        
        // Elementos de Diagnóstico
        const statusText = document.getElementById('status-text');
        const statusDot = document.getElementById('status-dot');
        const statusPing = document.querySelector('.animate-ping');
        const infoIp = document.getElementById('info-ip');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        // Proteção: Se o HTML ainda não carregou, não tenta atualizar
        if (!qrStatusText || !qrCodeDiv) return;

        // Atualiza campos de diagnóstico (se existirem na tela)
        if (infoIp) infoIp.innerText = data.ip || 'localhost';
        if (infoTimezone) infoTimezone.innerText = data.timezone || 'America/Sao_Paulo';
        if (infoEngine) infoEngine.innerText = data.engine || 'Whatsmeow';
        if (infoUptime) infoUptime.innerText = data.uptime || '0s';

        if (data.status === 'qr' && data.qr) {
            // Status: Aguardando leitura do QR Code
            qrStatusText.style.display = 'none';
            
            // Atualiza o monitoramento para "Aguardando QR Code"
            if (statusText) {
                statusText.innerText = "Aguardando QR Code";
                statusText.className = "text-xs font-semibold text-yellow-400 font-bold";
                statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-yellow-500";
                if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75";
            }

            // Só renderiza e roda a animação se o QR Code mudou
            if (data.qr !== lastQR) {
                lastQR = data.qr;
                const qrImageUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(data.qr)}`;
                qrCodeDiv.innerHTML = `
                    <div class="qr-container-wrapper">
                        <img src="${qrImageUrl}" alt="QR Code do WhatsApp" class="qr-animate">
                    </div>
                `;
            }
        } 
        else if (data.status === 'connected') {
            // Status: Conectado com sucesso
            qrCodeDiv.innerHTML = `
                <div class="flex flex-col items-center justify-center p-4">
                    <span class="text-6xl mb-3 animate-bounce">🎉</span>
                    <span class="text-emerald-500 font-bold text-lg">Sucesso!</span>
                </div>
            `;
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-green-600 font-bold text-sm">✅ Bot conectado com sucesso!</span>`;
            
            // Atualiza o monitoramento para "Conectado"
            if (statusText) {
                statusText.innerText = "Conectado";
                statusText.className = "text-xs font-semibold text-emerald-400 font-bold";
                statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-emerald-500";
                if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75";
            }

            // MATA O LOOP: O bot já conectou, não precisamos mais sobrecarregar a API
            clearInterval(statusInterval);
        }
        else if (data.status === 'waiting') {
            // Status: O servidor ligou, mas a biblioteca ainda não cuspiu o QR Code
            qrCodeDiv.innerHTML = `
                <div class="flex flex-col items-center justify-center p-4">
                    <span class="text-4xl animate-spin mb-3 text-indigo-400">🔄</span>
                </div>
            `;
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-gray-400 font-bold animate-pulse text-xs">Aguardando geração do QR Code...</span>`;

            // Atualiza o monitoramento para "Inicializando..."
            if (statusText) {
                statusText.innerText = "Inicializando...";
                statusText.className = "text-xs font-semibold text-cyan-400 font-bold";
                statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-cyan-500";
                if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75";
            }
        }
    } catch (error) {
        // Status: Erro ao tentar bater na API (Servidor offline ou reiniciando)
        console.error("Erro ao buscar status:", error);
        const qrStatusText = document.getElementById('qr-status');
        const statusText = document.getElementById('status-text');
        const statusDot = document.getElementById('status-dot');
        const statusPing = document.querySelector('.animate-ping');
        const infoIp = document.getElementById('info-ip');
        const infoTimezone = document.getElementById('info-timezone');
        const infoEngine = document.getElementById('info-engine');
        const infoUptime = document.getElementById('info-uptime');

        if (qrStatusText) {
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-red-500 font-bold text-xs">❌ Erro de conexão com a API</span>`;
        }

        if (statusText) {
            statusText.innerText = "Desconectado (Erro)";
            statusText.className = "text-xs font-semibold text-red-400 font-bold";
            statusDot.className = "relative inline-flex rounded-full h-2 w-2 bg-red-500";
            if (statusPing) statusPing.className = "animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75";
        }

        // Zera os diagnósticos em caso de erro na conexão
        if (infoIp) infoIp.innerText = 'Desconectado';
        if (infoTimezone) infoTimezone.innerText = 'Indisponível';
        if (infoEngine) infoEngine.innerText = 'Desconectado';
        if (infoUptime) infoUptime.innerText = 'Offline';
    }
}

// Inicia o loop para checar o status a cada 2 segundos
statusInterval = setInterval(checkStatus, 2000);

// Faz a primeira checagem imediatamente ao abrir a página
checkStatus();