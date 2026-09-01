let flowRoot = null;
let activeEditPath = [];
let activeEditMode = "edit";

async function fetchFlow() {
    try {
        const response = await fetch('/api/flow');
        if (!response.ok) throw new Error('Erro ao obter fluxo');
        flowRoot = await response.json();
        renderTree();
    } catch (error) {
        console.error(error);
        showToast('Erro ao obter o fluxo.', false);
    }
}

function renderTree() {
    const treeContainer = document.getElementById('flow-tree');
    treeContainer.innerHTML = '';

    if (!flowRoot) return;

    const rootEl = createNodeUI(flowRoot, [], true);
    treeContainer.appendChild(rootEl);
}

function createNodeUI(node, path, isRoot = false) {
    const div = document.createElement('div');
    div.className = `flex flex-col gap-2.5 rounded p-3 transition border ${isRoot ? 'bg-zinc-950 border-zinc-700' : 'bg-zinc-950 border-zinc-800 ml-4'}`;

    let badgeHTML = '';
    switch (node.type) {
        case 'menu':
            const backInfo = (isRoot || node.show_back_button !== false) ? ' (+Botão Voltar)' : ' (Sem Voltar)';
            badgeHTML = `<span class="text-[10px] font-bold bg-zinc-800 text-zinc-300 border border-zinc-700 px-1.5 py-0.5 rounded">Menu${backInfo}</span>`;
            break;
        case 'ticket':
            const attachmentTypes = [];
            if (node.ask_images !== false) attachmentTypes.push('Imagens');
            if (node.ask_docs !== false) attachmentTypes.push('Documentos');
            const attachmentsInfo = attachmentTypes.length > 0 ? ` (+Anexos: ${attachmentTypes.join('/')})` : ' (Sem Anexos)';
            badgeHTML = `<span class="text-[10px] font-bold bg-zinc-800 text-emerald-400 border border-zinc-700 px-1.5 py-0.5 rounded">Chamado GLPI (ID: ${node.glpi_id || 0})${attachmentsInfo}</span>`;
            break;
        case 'text':
            badgeHTML = '<span class="text-[10px] font-bold bg-zinc-800 text-cyan-400 border border-zinc-700 px-1.5 py-0.5 rounded">Resposta / FAQ</span>';
            break;
        case 'status':
            badgeHTML = '<span class="text-[10px] font-bold bg-zinc-800 text-purple-400 border border-zinc-700 px-1.5 py-0.5 rounded">Status Chamado</span>';
            break;
        case 'human':
            badgeHTML = '<span class="text-[10px] font-bold bg-zinc-800 text-amber-400 border border-zinc-700 px-1.5 py-0.5 rounded">Falar com Suporte</span>';
            break;
    }

    const pathStr = JSON.stringify(path);
    const parentPath = path.slice(0, -1);
    const childIdx = path[path.length - 1];

    const sortButtons = isRoot ? '' : `
        <button onclick="moveNode(${JSON.stringify(parentPath)}, ${childIdx}, -1)" class="px-1.5 py-0.5 hover:bg-zinc-800 rounded text-zinc-400 hover:text-zinc-100 text-xs">▲</button>
        <button onclick="moveNode(${JSON.stringify(parentPath)}, ${childIdx}, 1)" class="px-1.5 py-0.5 hover:bg-zinc-800 rounded text-zinc-400 hover:text-zinc-100 text-xs">▼</button>
    `;

    const addChildButton = node.type === 'menu' ? `
        <button onclick="openAddModal(${pathStr})" class="px-2 py-0.5 text-[11px] font-bold bg-zinc-800 text-zinc-200 hover:bg-zinc-700 rounded border border-zinc-700 transition">
            + Adicionar Opção
        </button>
    ` : '';

    const deleteButton = isRoot ? '' : `
        <button onclick="deleteNode(${JSON.stringify(parentPath)}, ${childIdx})" class="px-2 py-0.5 hover:bg-rose-950/60 text-rose-400 hover:text-rose-300 rounded text-xs border border-rose-900/60 font-semibold">Excluir</button>
    `;

    div.innerHTML = `
        <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-2">
                <span class="text-xs font-bold text-zinc-200">${node.title}</span>
                ${badgeHTML}
            </div>
            <div class="flex items-center gap-1.5">
                ${addChildButton}
                ${sortButtons}
                <button onclick="openEditModal(${pathStr})" class="px-2 py-0.5 hover:bg-zinc-800 rounded text-zinc-300 hover:text-zinc-100 text-xs font-semibold border border-zinc-800">Editar</button>
                ${deleteButton}
            </div>
        </div>
    `;

    if (node.type === 'menu' && node.children && node.children.length > 0) {
        const childrenDiv = document.createElement('div');
        childrenDiv.className = 'flex flex-col gap-2 border-l border-zinc-800 ml-1.5 mt-1 pt-1';

        node.children.forEach((child, idx) => {
            const childEl = createNodeUI(child, [...path, idx]);
            childrenDiv.appendChild(childEl);
        });

        div.appendChild(childrenDiv);
    } else if (node.type === 'menu') {
        const emptyDiv = document.createElement('div');
        emptyDiv.className = 'text-[11px] text-zinc-500 italic ml-4 mt-0.5';
        emptyDiv.innerText = '(Submenu sem opções ativas)';
        div.appendChild(emptyDiv);
    }

    return div;
}

function funcNodeByPath(path) {
    let curr = flowRoot;
    for (let idx of path) {
        curr = curr.children[idx];
    }
    return curr;
}

function openEditModal(path) {
    activeEditPath = path;
    activeEditMode = "edit";
    const node = funcNodeByPath(path);

    document.getElementById('modal-title').innerText = "Editar Opção";
    document.getElementById('node-title').value = node.title || '';
    document.getElementById('node-type').value = node.type || 'menu';
    document.getElementById('node-glpi').value = node.glpi_id || '';
    document.getElementById('node-content').value = node.content || '';

    document.getElementById('node-ask-images').checked = node.ask_images !== false;
    document.getElementById('node-ask-docs').checked = node.ask_docs !== false;
    document.getElementById('node-show-back').checked = node.show_back_button !== false;

    toggleModalFields();
    document.getElementById('edit-modal').classList.remove('hidden');
}

function openAddModal(parentPath) {
    activeEditPath = parentPath;
    activeEditMode = "add";

    document.getElementById('modal-title').innerText = "Nova Opção de Menu";
    document.getElementById('node-title').value = '';
    document.getElementById('node-type').value = 'menu';
    document.getElementById('node-glpi').value = '';
    document.getElementById('node-content').value = '';

    document.getElementById('node-ask-images').checked = true;
    document.getElementById('node-ask-docs').checked = true;
    document.getElementById('node-show-back').checked = true;

    toggleModalFields();
    document.getElementById('edit-modal').classList.remove('hidden');
}

function toggleModalFields() {
    const type = document.getElementById('node-type').value;
    const fGlpi = document.getElementById('field-glpi');
    const fAttach = document.getElementById('field-attachments');
    const fMenuOpts = document.getElementById('field-menu-options');
    const fContent = document.getElementById('field-content');
    const lblContent = document.getElementById('label-content');

    fGlpi.classList.add('hidden');
    fAttach.classList.add('hidden');
    fMenuOpts.classList.add('hidden');
    fContent.classList.add('hidden');

    if (type === 'ticket') {
        fGlpi.classList.remove('hidden');
        fAttach.classList.remove('hidden');
        fContent.classList.remove('hidden');
        lblContent.innerText = "Prompt Específico do Problema (Instrução para o usuário)";
    } else if (type === 'text') {
        fContent.classList.remove('hidden');
        lblContent.innerText = "Mensagem de Resposta Final / FAQ";
    } else if (type === 'menu') {
        fMenuOpts.classList.remove('hidden');
    }
}

function closeModal() {
    document.getElementById('edit-modal').classList.add('hidden');
}

function confirmModal() {
    const title = document.getElementById('node-title').value.trim();
    if (!title) {
        alert('Informe o título da opção.');
        return;
    }

    const type = document.getElementById('node-type').value;
    const glpiId = parseInt(document.getElementById('node-glpi').value) || 0;
    const content = document.getElementById('node-content').value.trim();

    const askImages = document.getElementById('node-ask-images').checked;
    const askDocs = document.getElementById('node-ask-docs').checked;
    const showBack = document.getElementById('node-show-back').checked;

    if (activeEditMode === "edit") {
        const node = funcNodeByPath(activeEditPath);
        node.title = title;
        node.type = type;

        if (type === 'ticket') {
            node.glpi_id = glpiId;
            node.content = content;
            node.ask_images = askImages;
            node.ask_docs = askDocs;
        } else if (type === 'text') {
            node.content = content;
        } else if (type === 'menu') {
            node.show_back_button = showBack;
            if (!node.children) node.children = [];
        }
    } else if (activeEditMode === "add") {
        const parentNode = funcNodeByPath(activeEditPath);
        if (!parentNode.children) parentNode.children = [];

        const newId = 'n_' + Date.now();
        const newNode = {
            id: newId,
            title: title,
            type: type
        };

        if (type === 'ticket') {
            newNode.glpi_id = glpiId;
            newNode.content = content;
            newNode.ask_images = askImages;
            newNode.ask_docs = askDocs;
        } else if (type === 'text') {
            newNode.content = content;
        } else if (type === 'menu') {
            newNode.show_back_button = showBack;
            newNode.children = [];
        }

        parentNode.children.push(newNode);
    }

    closeModal();
    renderTree();
}

function deleteNode(parentPath, idx) {
    if (!confirm('Deseja realmente remover esta opção e todos os seus submenus?')) return;
    const parent = funcNodeByPath(parentPath);
    parent.children.splice(idx, 1);
    renderTree();
}

function moveNode(parentPath, idx, direction) {
    const parent = funcNodeByPath(parentPath);
    const targetIdx = idx + direction;

    if (targetIdx < 0 || targetIdx >= parent.children.length) return;

    const temp = parent.children[idx];
    parent.children[idx] = parent.children[targetIdx];
    parent.children[targetIdx] = temp;

    renderTree();
}

async function saveFlow() {
    try {
        const response = await fetch('/api/flow', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(flowRoot)
        });

        if (!response.ok) throw new Error('Erro ao salvar fluxo');
        showToast('Árvore de fluxo salva com sucesso!', true);
    } catch (err) {
        console.error(err);
        showToast('Erro ao salvar o fluxo.', false);
    }
}

function showToast(message, isSuccess = true) {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    if (toastIcon) toastIcon.innerText = '';
    if (toastMsg) toastMsg.innerText = message;

    if (isSuccess) {
        toast.className = "fixed bottom-4 right-4 px-4 py-2 rounded bg-zinc-900 border border-zinc-700 text-emerald-400 font-semibold transition duration-200 flex items-center gap-2 z-50 text-xs";
    } else {
        toast.className = "fixed bottom-4 right-4 px-4 py-2 rounded bg-zinc-900 border border-zinc-700 text-rose-400 font-semibold transition duration-200 flex items-center gap-2 z-50 text-xs";
    }

    toast.classList.remove('translate-y-24', 'opacity-0');

    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

window.addEventListener('DOMContentLoaded', fetchFlow);
