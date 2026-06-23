let statusInterval;
let lastQR = "";

async function checkStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();
        
        const qrStatusText = document.getElementById('qr-status');
        const qrCodeDiv = document.getElementById('qrcode');

        // Proteção: Se o HTML ainda não carregou, não tenta atualizar
        if (!qrStatusText || !qrCodeDiv) return;

        if (data.status === 'qr' && data.qr) {
            // Status: Aguardando leitura do QR Code
            qrStatusText.style.display = 'none';
            
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
            qrCodeDiv.innerHTML = '';
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-green-600 font-bold text-lg">✅ Bot Conectado!</span>`;
            
            // MATA O LOOP: O bot já conectou, não precisamos mais sobrecarregar a API
            clearInterval(statusInterval);
        }
        else if (data.status === 'waiting') {
            // Status: O servidor ligou, mas a biblioteca ainda não cuspiu o QR Code
            qrCodeDiv.innerHTML = '';
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-gray-400 font-bold animate-pulse">Aguardando geração do QR Code...</span>`;
        }
    } catch (error) {
        // Status: Erro ao tentar bater na API (Servidor offline ou reiniciando)
        console.error("Erro ao buscar status:", error);
        const qrStatusText = document.getElementById('qr-status');
        if (qrStatusText) {
            qrStatusText.style.display = 'block';
            qrStatusText.innerHTML = `<span class="text-red-600 font-bold">❌ Erro de conexão com a API</span>`;
        }
    }
}

// Inicia o loop para checar o status a cada 2 segundos
statusInterval = setInterval(checkStatus, 2000);

// Faz a primeira checagem imediatamente ao abrir a página
checkStatus();