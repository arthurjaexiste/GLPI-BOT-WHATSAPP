let activeChatJID = null;
let activeChatName = "";
let activeChatStatus = null;
let activeChatAgent = null;
let chatsData = [];

function formatJIDToPhone(jid) {
    if (!jid) return "";
    let number = jid.split("@")[0].split(":")[0];
    if (number.length === 13 && number.startsWith("55")) {
        return `+${number.substring(0, 2)} (${number.substring(2, 4)}) ${number.substring(4, 9)}-${number.substring(9)}`;
    }
    if (number.length === 12 && number.startsWith("55")) {
        return `+${number.substring(0, 2)} (${number.substring(2, 4)}) ${number.substring(4, 8)}-${number.substring(8)}`;
    }
    return number;
}

function updateChatInputLockState(status, agentName) {
    const input = document.getElementById("chat-message-input");
    const btnSend = document.getElementById("btn-send-msg");
    const btnAudio = document.getElementById("btn-record-audio");
    const btnClose = document.getElementById("btn-close-chat");

    if (!input || !btnSend) return;

    if (status === "bot" || status === "queue") {
        input.disabled = true;
        input.placeholder = "Clique em 'Assumir Atendimento Humano' acima para responder...";
        btnSend.disabled = true;
        btnSend.classList.add("opacity-50", "pointer-events-none");
        if (btnAudio) {
            btnAudio.disabled = true;
            btnAudio.classList.add("opacity-50", "pointer-events-none");
        }
        if (btnClose) btnClose.style.display = "none";
    } else {
        input.disabled = false;
        input.placeholder = "Digite a resposta do suporte...";
        btnSend.disabled = false;
        btnSend.classList.remove("opacity-50", "pointer-events-none");
        if (btnAudio) {
            btnAudio.disabled = false;
            btnAudio.classList.remove("opacity-50", "pointer-events-none");
        }
        if (btnClose) btnClose.style.display = "inline-flex";
    }
}

async function loadChatsList(isBackgroundRefresh = false) {
    try {
        const response = await fetch("/api/chats");
        if (!response.ok) throw new Error("Falha ao carregar lista de conversas");

        chatsData = await response.json();
        
        if (!chatsData || chatsData.length === 0) {
            document.getElementById("chats-list-container").innerHTML = `
                <div class="p-4 text-center text-xs text-zinc-500 italic">Nenhum chamado ou conversa ativa no momento.</div>
            `;
            return;
        }

        const currentSearch = document.getElementById("chat-search-input")?.value.trim().toLowerCase();
        if (currentSearch) {
            filterChats();
        } else {
            renderChatsList();
        }

        if (activeChatJID) {
            const currentChat = chatsData.find(c => c.jid === activeChatJID);
            if (currentChat) {
                activeChatStatus = currentChat.status;
                activeChatAgent = currentChat.agent_name;
                updateChatInputLockState(activeChatStatus, activeChatAgent);
            }
        }
    } catch (error) {
        console.error(error);
        if (!isBackgroundRefresh) {
            showToast("Erro ao carregar conversas", false);
        }
    }
}

function renderChatsList(filteredData = null) {
    const container = document.getElementById("chats-list-container");
    if (!container) return;

    const data = filteredData || chatsData;

    if (data.length === 0) {
        container.innerHTML = `<div class="text-center py-6 text-zinc-500 text-xs italic">Nenhuma conversa encontrada.</div>`;
        return;
    }

    container.innerHTML = "";
    data.forEach(chat => {
        let statusText = "Bot";
        let statusClass = "status-bot";
        if (chat.status === "live_chat") {
            statusText = chat.agent_name ? `Suporte (${chat.agent_name})` : "Suporte";
            statusClass = "status-live_chat";
        } else if (chat.status === "queue") {
            statusText = "Fila";
            statusClass = "status-queue";
        }

        const isSelected = activeChatJID === chat.jid;
        const cardClass = isSelected
            ? "bg-zinc-800 border-zinc-700"
            : "bg-zinc-950 hover:bg-zinc-800/60 border-zinc-800/80";

        const card = document.createElement("div");
        card.className = `p-2.5 rounded border cursor-pointer transition flex flex-col gap-1 relative group ${cardClass}`;
        card.onclick = () => selectChat(chat.jid, chat.name, chat.status, chat.agent_name);

        let snippet = chat.last_message || "Nenhuma mensagem...";
        if (snippet.length > 35) snippet = snippet.substring(0, 35) + "...";

        const phoneFormatted = formatJIDToPhone(chat.jid);
        const displayName = chat.name ? chat.name : phoneFormatted;
        const subText = chat.name ? `<span class="text-[10px] text-zinc-400 font-mono">${phoneFormatted}</span>` : '';

        card.innerHTML = `
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 min-w-0">
                    <img src="/api/chats/avatar?jid=${encodeURIComponent(chat.jid)}&name=${encodeURIComponent(displayName)}" class="w-7 h-7 rounded object-cover border border-zinc-800 shrink-0" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=27272a&color=fff';" />
                    <div class="flex flex-col min-w-0">
                        <span class="text-xs font-bold text-zinc-200 truncate max-w-[130px]">${displayName}</span>
                        ${subText}
                    </div>
                </div>
                <div class="flex items-center gap-1 shrink-0">
                    <span class="status-badge ${statusClass} group-hover:hidden">${statusText}</span>
                    <button onclick="event.stopPropagation(); deleteChat('${chat.jid}');" class="hidden group-hover:inline-flex items-center justify-center text-rose-400 hover:text-rose-300 transition text-[10px] px-1.5 py-0.5 border border-rose-900/60 rounded font-semibold bg-rose-950/40" title="Apagar conversa">
                        Apagar
                    </button>
                </div>
            </div>
            <div class="flex items-center justify-between gap-2 mt-1">
                <span class="text-[11px] text-zinc-400 truncate flex-1">${snippet}</span>
                <span class="text-[10px] text-zinc-500 font-mono">${chat.timestamp}</span>
            </div>
        `;
        container.appendChild(card);
    });
}

function filterChats() {
    const term = document.getElementById("chat-search-input").value.toLowerCase();
    if (!term) {
        renderChatsList();
        return;
    }
    const filtered = chatsData.filter(chat => {
        return (chat.name && chat.name.toLowerCase().includes(term)) ||
            chat.jid.toLowerCase().includes(term) ||
            (chat.last_message && chat.last_message.toLowerCase().includes(term));
    });
    renderChatsList(filtered);
}

async function selectChat(jid, name, status, agentName) {
    activeChatJID = jid;
    activeChatName = name || jid;
    activeChatStatus = status;
    activeChatAgent = agentName;

    updateChatInputLockState(status, agentName);

    document.getElementById("chat-placeholder").style.display = "none";
    document.getElementById("active-chat-header").classList.remove("hidden");
    document.getElementById("chat-messages-container").classList.remove("hidden");
    document.getElementById("chat-input-container").classList.remove("hidden");

    const phoneFormatted = formatJIDToPhone(jid);
    const displayName = name ? name : phoneFormatted;

    document.getElementById("active-chat-name").textContent = displayName;
    document.getElementById("active-chat-jid").textContent = phoneFormatted;

    const avatarContainer = document.getElementById("active-chat-avatar");
    avatarContainer.innerHTML = `<img src="/api/chats/avatar?jid=${encodeURIComponent(jid)}&name=${encodeURIComponent(displayName)}" class="w-full h-full rounded object-cover" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=27272a&color=fff';" />`;

    const statusDot = document.getElementById("active-chat-status-dot");
    const statusText = document.getElementById("active-chat-status-text");

    statusDot.className = "w-1.5 h-1.5 rounded-full";
    if (status === "live_chat") {
        statusDot.classList.add("bg-emerald-500");
        statusText.textContent = agentName ? `Suporte (${agentName})` : "Suporte";
        document.getElementById("assume-chat-banner").style.display = "none";
    } else if (status === "queue") {
        statusDot.classList.add("bg-amber-500");
        statusText.textContent = "Fila de Espera";
        document.getElementById("assume-chat-banner").style.display = "flex";
    } else {
        statusDot.classList.add("bg-zinc-500");
        statusText.textContent = "Bot";
        document.getElementById("assume-chat-banner").style.display = "flex";
    }

    renderChatsList();
    await refreshActiveMessages(true);
}

let lastMessagesCount = 0;
async function refreshActiveMessages(forceScroll = false) {
    if (!activeChatJID) return;

    try {
        const response = await fetch(`/api/chats/messages?jid=${encodeURIComponent(activeChatJID)}`);
        if (!response.ok) throw new Error("Erro ao buscar histórico");

        const messages = await response.json();
        renderMessages(messages, forceScroll);
    } catch (error) {
        console.error("Erro de sincronização de mensagens:", error);
    }
}

let activeReplyQuote = null;
let selectedMediaFile = null;

function handleFileSelected(input) {
    const files = input.files;
    if (!files || files.length === 0) return;

    selectedMediaFile = files[0];
    const preview = document.getElementById("media-attachment-preview");
    const imgEl = document.getElementById("media-preview-img");
    const iconEl = document.getElementById("media-preview-icon");
    const nameEl = document.getElementById("media-preview-filename");
    const sizeEl = document.getElementById("media-preview-size");

    if (nameEl) nameEl.textContent = selectedMediaFile.name;
    if (sizeEl) sizeEl.textContent = Math.round(selectedMediaFile.size / 1024) + " KB";

    if (selectedMediaFile.type.startsWith("image/")) {
        const reader = new FileReader();
        reader.onload = (e) => {
            if (imgEl) {
                imgEl.src = e.target.result;
                imgEl.classList.remove("hidden");
            }
            if (iconEl) iconEl.classList.add("hidden");
        };
        reader.readAsDataURL(selectedMediaFile);
    } else {
        if (imgEl) imgEl.classList.add("hidden");
        if (iconEl) {
            iconEl.textContent = selectedMediaFile.name.split('.').pop().toUpperCase() || "DOC";
            iconEl.classList.remove("hidden");
        }
    }

    if (preview) preview.classList.remove("hidden");
}

function clearSelectedMedia() {
    selectedMediaFile = null;
    const fileInput = document.getElementById("chat-file-input");
    if (fileInput) fileInput.value = "";
    const preview = document.getElementById("media-attachment-preview");
    if (preview) preview.classList.add("hidden");
}

let mediaRecorder = null;
let audioChunks = [];
let recordingTimerInterval = null;
let recordingSeconds = 0;

async function toggleAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        stopAndSendAudioRecording();
        return;
    }

    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        mediaRecorder = new MediaRecorder(stream);
        audioChunks = [];

        mediaRecorder.ondataavailable = event => {
            if (event.data.size > 0) audioChunks.push(event.data);
        };

        mediaRecorder.onstop = async function () {
            stream.getTracks().forEach(track => track.stop());
            clearInterval(recordingTimerInterval);
            const recBar = document.getElementById("audio-recording-bar");
            if (recBar) recBar.classList.add("hidden");

            if (audioChunks.length > 0) {
                const audioBlob = new Blob(audioChunks, { type: "audio/ogg; codecs=opus" });
                const audioFile = new File([audioBlob], `audio_${Date.now()}.ogg`, { type: "audio/ogg" });
                
                selectedMediaFile = audioFile;
                await sendConsoleMessage();
            }
        };

        mediaRecorder.start();
        recordingSeconds = 0;
        const timerEl = document.getElementById("recording-timer");
        if (timerEl) timerEl.textContent = "00:00";
        const recBar = document.getElementById("audio-recording-bar");
        if (recBar) recBar.classList.remove("hidden");

        recordingTimerInterval = setInterval(() => {
            recordingSeconds++;
            const mins = String(Math.floor(recordingSeconds / 60)).padStart(2, '0');
            const secs = String(recordingSeconds % 60).padStart(2, '0');
            if (timerEl) timerEl.textContent = `${mins}:${secs}`;
        }, 1000);

    } catch (err) {
        console.error(err);
        showToast("Permissão de microfone negada ou indisponível.", false);
    }
}

function cancelAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        audioChunks = [];
        mediaRecorder.stop();
    }
    clearInterval(recordingTimerInterval);
    const recBar = document.getElementById("audio-recording-bar");
    if (recBar) recBar.classList.add("hidden");
}

function stopAndSendAudioRecording() {
    if (mediaRecorder && mediaRecorder.state === "recording") {
        mediaRecorder.stop();
    }
}

function replyToMessage(senderName, originalText, waMsgId = '', senderJid = '') {
    let cleanName = senderName;
    if (!cleanName || cleanName === "Contato" || cleanName.startsWith("17188") || (cleanName.length >= 10 && !isNaN(cleanName))) {
        cleanName = activeChatName || formatJIDToPhone(activeChatJID);
    }
    activeReplyQuote = {
        name: cleanName,
        text: originalText || "",
        quoted_id: waMsgId || "",
        quoted_jid: senderJid || ""
    };

    const preview = document.getElementById("reply-quote-preview");
    const nameEl = document.getElementById("reply-quote-name");
    const textEl = document.getElementById("reply-quote-text");
    const input = document.getElementById("chat-message-input");

    if (preview && nameEl && textEl) {
        nameEl.textContent = activeReplyQuote.name;
        let snippet = activeReplyQuote.text;
        if (snippet.length > 60) snippet = snippet.substring(0, 60) + "...";
        textEl.textContent = snippet || "Mensagem em mídia";
        preview.classList.remove("hidden");
    }

    if (input) input.focus();
}

function cancelReplyQuote() {
    activeReplyQuote = null;
    const preview = document.getElementById("reply-quote-preview");
    if (preview) preview.classList.add("hidden");
}

function escapeHTML(str) {
    if (!str) return "";
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

function escapeQuotes(str) {
    if (!str) return "";
    return str.replace(/'/g, "\\'").replace(/"/g, '\\"');
}

function renderMessages(messages, forceScroll = false) {
    const container = document.getElementById("chat-messages-container");

    if (messages.length === lastMessagesCount && !forceScroll) {
        return;
    }

    container.innerHTML = "";
    lastMessagesCount = messages.length;

    if (messages.length === 0) {
        container.innerHTML = `<div class="text-center py-6 text-zinc-500 text-xs italic">Nenhuma mensagem nesta conversa.</div>`;
        return;
    }

    messages.forEach(msg => {
        const row = document.createElement("div");
        row.className = `w-full flex ${msg.is_from_me ? 'justify-end' : 'justify-start'} items-center gap-2 my-1 group`;

        const bubble = document.createElement("div");
        const bubbleStyle = msg.is_from_me 
            ? 'bg-zinc-800 border border-zinc-700 text-zinc-100 rounded px-3 py-2 max-w-xl text-xs' 
            : 'bg-zinc-900 border border-zinc-800 text-zinc-200 rounded px-3 py-2 max-w-xl text-xs';
        bubble.className = bubbleStyle;

        let quoteHTML = "";
        if (msg.reply_to_text) {
            let authorName = msg.reply_to_name || 'Contato';
            if (!authorName || authorName === 'Contato' || authorName.startsWith("17188") || (authorName.length >= 10 && !isNaN(authorName))) {
                authorName = activeChatName || formatJIDToPhone(activeChatJID);
            }
            quoteHTML = `
                <div class="bg-zinc-950 border-l-2 border-zinc-600 p-1.5 mb-2 rounded text-[11px] text-zinc-300">
                    <div class="font-bold text-zinc-100">${escapeHTML(authorName)}</div>
                    <div class="truncate text-zinc-400">${escapeHTML(msg.reply_to_text)}</div>
                </div>
            `;
        }

        let mediaHTML = "";
        let textContent = msg.text || '';

        if (msg.media_url) {
            const lowerUrl = msg.media_url.toLowerCase();
            const isBase64Img = lowerUrl.startsWith("data:image/");
            const isBase64Audio = lowerUrl.startsWith("data:audio/");
            const isBase64Doc = lowerUrl.startsWith("data:application/") || lowerUrl.startsWith("data:text/");

            if (msg.type === "image" || isBase64Img || lowerUrl.endsWith(".jpg") || lowerUrl.endsWith(".jpeg") || lowerUrl.endsWith(".png") || lowerUrl.endsWith(".webp") || lowerUrl.endsWith(".gif")) {
                mediaHTML = `<div class="mb-2 overflow-hidden rounded">
                    <img src="${msg.media_url}" alt="Imagem" class="w-full max-h-72 object-cover rounded border border-zinc-800 cursor-pointer" onclick="window.open('${msg.media_url}', '_blank')" />
                </div>`;
            } else if (msg.type === "audio" || isBase64Audio || lowerUrl.endsWith(".ogg") || lowerUrl.endsWith(".mp3") || lowerUrl.endsWith(".m4a") || lowerUrl.endsWith(".wav") || lowerUrl.endsWith(".webm")) {
                mediaHTML = `<div class="mb-1 py-1">
                    <audio controls src="${msg.media_url}" class="max-w-[240px] h-8 focus:outline-none"></audio>
                </div>`;
            } else if (msg.type === "document" || isBase64Doc || lowerUrl.endsWith(".pdf") || lowerUrl.endsWith(".doc") || lowerUrl.endsWith(".docx") || lowerUrl.endsWith(".txt") || lowerUrl.endsWith(".zip")) {
                mediaHTML = `<div class="mb-2">
                    <a href="${msg.media_url}" download="documento" target="_blank" class="inline-flex items-center gap-1.5 px-2.5 py-1 bg-zinc-950 border border-zinc-800 rounded text-zinc-200 font-semibold text-xs hover:border-zinc-700 transition">
                        Baixar / Abrir Documento Anexo
                    </a>
                </div>`;
            }
            if (textContent.startsWith("[Imagem]") || textContent.startsWith("[Áudio]") || textContent.startsWith("[Documento] audio_") || textContent.startsWith("[Documento] voice_")) {
                if (msg.type === "audio" || isBase64Audio || lowerUrl.endsWith(".ogg") || lowerUrl.endsWith(".mp3") || lowerUrl.endsWith(".m4a") || lowerUrl.endsWith(".wav") || lowerUrl.endsWith(".webm")) {
                    textContent = "";
                } else {
                    textContent = textContent.replace("[Imagem]", "").replace("[Áudio]", "").trim();
                }
            }
        } else if (msg.type === "image" || textContent.includes("[Imagem]")) {
            mediaHTML = `<div class="mb-1 text-[11px] font-semibold text-zinc-300 bg-zinc-950 px-2 py-1 rounded border border-zinc-800">
                Imagem enviada no WhatsApp
            </div>`;
            if (textContent === "[Imagem]") textContent = "";
            else textContent = textContent.replace(/\[Imagem\]/g, "").trim();
        } else if (msg.type === "audio" || textContent.includes("[Áudio]")) {
            mediaHTML = `<div class="mb-1 text-[11px] font-semibold text-zinc-300 bg-zinc-950 px-2 py-1 rounded border border-zinc-800">
                Áudio de Voz do WhatsApp
            </div>`;
            if (textContent === "[Áudio]") textContent = "";
        }

        const formattedText = textContent ? textContent.replace(/\n/g, "<br>") : "";

        const bubbleHTML = `
            ${quoteHTML}
            ${mediaHTML}
            ${formattedText ? `<div class="font-normal leading-relaxed">${formattedText}</div>` : ''}
            <div class="text-[10px] mt-1 text-right font-mono ${msg.is_from_me ? 'text-zinc-400' : 'text-zinc-500'}">${msg.timestamp}</div>
        `;

        bubble.innerHTML = bubbleHTML;

        const replyBtn = !msg.is_from_me ? `
            <button onclick="replyToMessage('${escapeQuotes(msg.sender_name || activeChatName)}', '${escapeQuotes(msg.text)}', '${escapeQuotes(msg.wa_message_id || '')}', '${escapeQuotes(msg.sender_jid || '')}')" class="opacity-0 group-hover:opacity-100 transition-opacity text-[11px] text-zinc-400 hover:text-zinc-100 bg-zinc-900 border border-zinc-800 px-2 py-0.5 rounded font-semibold cursor-pointer" title="Responder">
                Responder
            </button>
        ` : '';

        if (msg.is_from_me) {
            row.appendChild(bubble);
        } else {
            row.appendChild(bubble);
            row.appendChild(document.createRange().createContextualFragment(replyBtn));
        }

        container.appendChild(row);
    });

    if (forceScroll || container.scrollTop + container.clientHeight >= container.scrollHeight - 150) {
        container.scrollTop = container.scrollHeight;
    }
}

async function sendConsoleMessage() {
    const input = document.getElementById("chat-message-input");
    const text = input ? input.value.trim() : "";

    if (!activeChatJID) {
        showToast("Nenhuma conversa selecionada.", false);
        return;
    }

    if (!text && !selectedMediaFile) {
        return;
    }

    const btnSend = document.getElementById("btn-send-msg");
    if (btnSend) btnSend.disabled = true;

    try {
        let response;
        if (selectedMediaFile) {
            const formData = new FormData();
            formData.append("jid", activeChatJID);
            if (text) formData.append("caption", text);
            formData.append("file", selectedMediaFile);

            if (activeReplyQuote) {
                if (activeReplyQuote.name) formData.append("reply_to_name", activeReplyQuote.name);
                if (activeReplyQuote.text) formData.append("reply_to_text", activeReplyQuote.text);
                if (activeReplyQuote.quoted_id) formData.append("quoted_id", activeReplyQuote.quoted_id);
                if (activeReplyQuote.quoted_jid) formData.append("quoted_jid", activeReplyQuote.quoted_jid);
            }

            response = await fetch("/api/chats/send-media", {
                method: "POST",
                body: formData
            });
        } else {
            const payload = {
                jid: activeChatJID,
                text: text,
                reply_to_name: activeReplyQuote ? (activeReplyQuote.name || "") : "",
                reply_to_text: activeReplyQuote ? (activeReplyQuote.text || "") : "",
                quoted_id: activeReplyQuote ? (activeReplyQuote.quoted_id || "") : "",
                quoted_jid: activeReplyQuote ? (activeReplyQuote.quoted_jid || "") : ""
            };

            response = await fetch("/api/chats/send", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify(payload)
            });
        }

        if (!response.ok) {
            const errData = await response.json().catch(() => ({}));
            throw new Error(errData.message || errData.error || "Erro ao enviar mensagem");
        }

        if (input) input.value = "";
        clearSelectedMedia();
        cancelReplyQuote();

        await refreshActiveMessages(true);
        loadChatsList(true);
    } catch (error) {
        console.error(error);
        showToast(error.message || "Falha ao enviar mensagem.", false);
    } finally {
        if (btnSend) btnSend.disabled = false;
    }
}

function handleInputKey(e) {
    if (e.key === "Enter") {
        sendConsoleMessage();
    }
}

function showToast(message, isSuccess = true) {
    const toast = document.getElementById("toast");
    const icon = document.getElementById("toast-icon");
    const msg = document.getElementById("toast-message");

    if (icon) icon.textContent = "";
    if (msg) msg.textContent = message;

    toast.className = `fixed bottom-4 right-4 px-4 py-2 rounded bg-zinc-900 border text-xs font-semibold flex items-center gap-2 z-50 transition duration-200 ${isSuccess
            ? 'border-zinc-700 text-emerald-400'
            : 'border-zinc-700 text-rose-400'
        }`;

    toast.classList.remove("translate-y-24", "opacity-0");

    setTimeout(() => {
        toast.classList.add("translate-y-24", "opacity-0");
    }, 3000);
}

function clearActiveChatUI() {
    activeChatJID = null;
    activeChatName = "";
    activeChatStatus = null;
    lastMessagesCount = 0;

    const placeholder = document.getElementById("chat-placeholder");
    const container = document.getElementById("chat-messages-container");
    const header = document.getElementById("active-chat-header");
    const inputContainer = document.getElementById("chat-input-container");

    if (placeholder) placeholder.style.display = "flex";
    if (container) container.classList.add("hidden");
    if (header) header.classList.add("hidden");
    if (inputContainer) inputContainer.classList.add("hidden");

    clearSelectedMedia();
    cancelReplyQuote();
}

async function deleteChat(jid) {
    if (!confirm("Tem certeza que deseja apagar esta conversa e todo o histórico?")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/delete?jid=${encodeURIComponent(jid)}`, {
            method: "DELETE"
        });

        if (!response.ok) throw new Error("Erro ao apagar conversa");

        showToast("Conversa apagada com sucesso!");

        const deletedNum = formatJIDToPhone(jid);
        const activeNum = activeChatJID ? formatJIDToPhone(activeChatJID) : "";

        if (!activeChatJID || activeChatJID === jid || deletedNum === activeNum) {
            clearActiveChatUI();
        }

        await loadChatsList();
    } catch (error) {
        console.error(error);
        showToast("Falha ao apagar conversa.", false);
    }
}

async function deleteActiveChat() {
    if (!activeChatJID) return;
    await deleteChat(activeChatJID);
}

async function closeLiveChat() {
    if (!activeChatJID) return;
    if (!confirm("Deseja finalizar o atendimento humano deste chamado e devolver para o bot?")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/close?jid=${encodeURIComponent(activeChatJID)}`, {
            method: "POST"
        });

        if (!response.ok) {
            const data = await response.json().catch(() => null);
            throw new Error((data && data.message) || "Erro ao finalizar atendimento");
        }

        showToast("Atendimento finalizado com sucesso.");
        activeChatStatus = "bot";
        activeChatAgent = null;
        updateChatInputLockState("bot", null);
        document.getElementById("assume-chat-banner").style.display = "flex";

        loadChatsList();

        const statusDot = document.getElementById("active-chat-status-dot");
        const statusText = document.getElementById("active-chat-status-text");
        if (statusDot && statusText) {
            statusDot.className = "w-1.5 h-1.5 rounded-full bg-zinc-500";
            statusText.textContent = "Bot";
        }
    } catch (error) {
        console.error(error);
        showToast(error.message || "Falha ao finalizar atendimento.", false);
    }
}

async function assumeLiveChat() {
    if (!activeChatJID) return;

    try {
        const response = await fetch('/api/chats/assume', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                jid: activeChatJID
            })
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(data.message || "Erro ao assumir atendimento");
        }

        showToast("Atendimento assumido com sucesso.");
        document.getElementById("assume-chat-banner").style.display = "none";

        activeChatStatus = "live_chat";
        const statusDot = document.getElementById("active-chat-status-dot");
        const statusText = document.getElementById("active-chat-status-text");
        if (statusDot && statusText) {
            statusDot.className = "w-1.5 h-1.5 rounded-full bg-emerald-500";
            statusText.textContent = data.agent ? `Suporte (${data.agent})` : "Suporte";
        }

        await loadChatsList();
        refreshActiveMessages(true);
    } catch (error) {
        console.error(error);
        showToast("Falha ao assumir atendimento.", false);
    }
}

document.addEventListener("DOMContentLoaded", () => {
    loadChatsList();
    setInterval(() => {
        loadChatsList(true);
        if (activeChatJID) {
            refreshActiveMessages();
        }
    }, 3000);
});
