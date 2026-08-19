let token = localStorage.getItem('token');
let currentPage = 'dashboard';
let dbConfigs = [];
let dataCompareResults = null;
let schemaSourceTables = [];
const DATA_CHANGE_TYPE = {
    INSERT: 'insert',
    UPDATE: 'update',
    DELETE: 'delete'
};

const DATA_CHANGE_TYPE_CONFIG = {
    [DATA_CHANGE_TYPE.INSERT]: { label: '新增', icon: 'fa-plus-circle', colorClass: 'text-green-700', backgroundClass: 'bg-green-50', borderClass: 'border-green-200' },
    [DATA_CHANGE_TYPE.UPDATE]: { label: '修改', icon: 'fa-pencil-square-o', colorClass: 'text-blue-700', backgroundClass: 'bg-blue-50', borderClass: 'border-blue-200' },
    [DATA_CHANGE_TYPE.DELETE]: { label: '删除', icon: 'fa-trash', colorClass: 'text-red-700', backgroundClass: 'bg-red-50', borderClass: 'border-red-200' }
};

const COMPARE_LOG_MAX_SIZE = 100;
const compareLogState = {
    schema: { containerID: 'schemaCompareLogs', entries: [], renderScheduled: false },
    data: { containerID: 'dataCompareLogs', entries: [], renderScheduled: false }
};

async function init() {
    if (token) {
        const valid = await validateToken();
        if (valid) renderApp();
        else renderLogin();
    } else {
        renderLogin();
    }
}

async function validateToken() {
    try {
        const response = await fetch('/api/auth/validate', { headers: { 'Authorization': `Bearer ${token}` } });
        const result = await response.json();
        return result.valid;
    } catch { return false; }
}

function renderLogin() {
    document.getElementById('app').innerHTML = `
        <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
            <div class="modal-content bg-white rounded-2xl shadow-2xl p-8 w-full max-w-sm mx-4" style="box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);">
                <div class="text-center mb-8">
                    <div class="flex justify-center mb-4">
                        <div class="w-16 h-16 bg-gradient-to-br from-blue-500 to-blue-600 rounded-2xl flex items-center justify-center shadow-lg shadow-blue-500/20">
                            <i class="fa fa-database text-white text-2xl"></i>
                        </div>
                    </div>
                    <h1 class="text-xl font-bold text-gray-900">数据对比工具</h1>
                    <p class="text-sm text-gray-500 mt-1">请登录以继续</p>
                </div>
                <form id="loginForm" class="space-y-4">
                    <div>
                        <label class="label">用户名</label>
                        <input type="text" name="username" class="input" placeholder="请输入用户名" autocomplete="username">
                    </div>
                    <div>
                        <label class="label">密码</label>
                        <input type="password" name="password" class="input" placeholder="请输入密码" autocomplete="current-password">
                    </div>
                    <button type="submit" class="btn btn-primary w-full btn-lg">
                        <i class="fa fa-sign-in"></i>登录
                    </button>
                </form>
                <div id="loginError" class="mt-4 text-sm text-red-600 text-center hidden"></div>
                <p class="mt-6 text-center text-xs text-gray-400">默认账号: admin / admin123</p>
            </div>
        </div>
    `;
    document.getElementById('loginForm').addEventListener('submit', handleLogin);
}

async function handleLogin(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const username = formData.get('username');
    const password = formData.get('password');
    const errorDiv = document.getElementById('loginError');
    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        const result = await response.json();
        if (response.ok) {
            token = result.token;
            localStorage.setItem('token', token);
            renderApp();
        } else {
            errorDiv.textContent = result.error || '登录失败';
            errorDiv.classList.remove('hidden');
        }
    } catch (err) {
        errorDiv.textContent = '网络错误: ' + err.message;
        errorDiv.classList.remove('hidden');
    }
}

function renderApp() {
    const navItems = [
        { id: 'dashboard', icon: 'fa-home', label: '仪表盘' },
        { id: 'dbconfig', icon: 'fa-cog', label: '数据库配置' },
        { id: 'schemacompare', icon: 'fa-table', label: '表结构比对' },
        { id: 'datacompare', icon: 'fa-exchange', label: '数据对比' }
    ];
    document.getElementById('app').innerHTML = `
        <div class="flex h-screen overflow-hidden">
            <aside class="sidebar w-60 flex-shrink-0 flex flex-col">
                <div class="flex items-center gap-3 px-5 py-5 border-b border-slate-200">
                    <div class="w-9 h-9 bg-blue-600 rounded-lg flex items-center justify-center shadow-sm">
                        <i class="fa fa-database text-white text-sm"></i>
                    </div>
                    <div>
                        <span class="font-semibold text-sm">数据对比工具</span>
                        <p class="text-xs text-slate-400 mt-0.5">Database Compare</p>
                    </div>
                </div>
                <nav class="flex-1 py-3 space-y-0.5">
                    ${navItems.map(item => `
                        <a href="#" onclick="navigate('${item.id}')" class="nav-link flex items-center gap-3 px-4 py-2.5 text-sm ${currentPage === item.id ? 'active' : 'text-slate-600'}" style="margin: 2px 8px;">
                            <i class="fa ${item.icon} w-5 text-center"></i>
                            <span>${item.label}</span>
                        </a>
                    `).join('')}
                </nav>
                <div class="p-4 border-t border-slate-200">
                    <button onclick="logout()" class="flex items-center gap-3 px-4 py-2.5 text-sm text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-lg w-full transition">
                        <i class="fa fa-sign-out w-5 text-center"></i>
                        <span>退出登录</span>
                    </button>
                </div>
            </aside>
            <main class="flex-1 overflow-auto bg-surface-50">
                <div class="max-w-7xl mx-auto px-8 py-6">
                    <div id="mainContent"></div>
                </div>
            </main>
        </div>
    `;
    renderMainContent();
}

function logout() {
    token = null;
    localStorage.removeItem('token');
    renderLogin();
}

function navigate(page) {
    currentPage = page;
    renderApp();
}

async function renderMainContent() {
    const content = document.getElementById('mainContent');
    switch (currentPage) {
        case 'dashboard': renderDashboard(content); break;
        case 'dbconfig': await renderDBConfig(content); break;
        case 'schemacompare': await renderSchemaCompare(content); break;
        case 'datacompare': await renderDataCompare(content); break;
    }
}

function renderDashboard(content) {
    content.innerHTML = `
        <div class="mb-8">
                    <h1 class="text-xl font-bold text-slate-900">仪表盘</h1>
            <p class="text-sm text-slate-500 mt-1">欢迎使用数据对比工具</p>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8">
            <div class="card p-5 stat-card">
                <div class="flex items-center gap-4">
                    <div class="w-12 h-12 bg-gradient-to-br from-blue-500 to-blue-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/20">
                        <i class="fa fa-database text-white text-lg"></i>
                    </div>
                    <div>
                        <p class="text-sm text-slate-500">数据库配置</p>
                        <p class="text-2xl font-bold text-slate-900 mt-0.5">${dbConfigs.length}</p>
                    </div>
                </div>
                <i class="fa fa-database stat-icon text-slate-200"></i>
            </div>
            <div class="card p-5 stat-card">
                <div class="flex items-center gap-4">
                    <div class="w-12 h-12 bg-gradient-to-br from-violet-500 to-violet-600 rounded-xl flex items-center justify-center shadow-lg shadow-violet-500/20">
                        <i class="fa fa-table text-white text-lg"></i>
                    </div>
                    <div>
                        <p class="text-sm text-slate-500">表结构比对</p>
                        <p class="text-2xl font-bold text-slate-900 mt-0.5">就绪</p>
                    </div>
                </div>
                <i class="fa fa-table stat-icon text-slate-200"></i>
            </div>
            <div class="card p-5 stat-card">
                <div class="flex items-center gap-4">
                    <div class="w-12 h-12 bg-gradient-to-br from-emerald-500 to-emerald-600 rounded-xl flex items-center justify-center shadow-lg shadow-emerald-500/20">
                        <i class="fa fa-exchange text-white text-lg"></i>
                    </div>
                    <div>
                        <p class="text-sm text-slate-500">数据对比</p>
                        <p class="text-2xl font-bold text-slate-900 mt-0.5">就绪</p>
                    </div>
                </div>
                <i class="fa fa-exchange stat-icon text-slate-200"></i>
            </div>
        </div>
        <div class="card p-6">
            <h2 class="text-base font-semibold text-slate-900 mb-4">快速开始</h2>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                <button onclick="navigate('dbconfig')" class="card-hover card p-5 flex flex-col items-center text-center gap-2 cursor-pointer border-2 border-dashed border-slate-200 hover:border-blue-300 hover:bg-blue-50/50">
                    <i class="fa fa-cog text-blue-500 text-2xl"></i>
                    <div>
                        <span class="font-medium text-slate-800">添加数据库配置</span>
                        <p class="text-xs text-slate-500 mt-0.5">配置源和目标数据库连接</p>
                    </div>
                </button>
                <button onclick="navigate('schemacompare')" class="card-hover card p-5 flex flex-col items-center text-center gap-2 cursor-pointer border-2 border-dashed border-slate-200 hover:border-violet-300 hover:bg-violet-50/50">
                    <i class="fa fa-table text-violet-500 text-2xl"></i>
                    <div>
                        <span class="font-medium text-slate-800">表结构比对</span>
                        <p class="text-xs text-slate-500 mt-0.5">比对表结构差异并生成 SQL</p>
                    </div>
                </button>
                <button onclick="navigate('datacompare')" class="card-hover card p-5 flex flex-col items-center text-center gap-2 cursor-pointer border-2 border-dashed border-slate-200 hover:border-emerald-300 hover:bg-emerald-50/50">
                    <i class="fa fa-exchange text-emerald-500 text-2xl"></i>
                    <div>
                        <span class="font-medium text-slate-800">开始数据对比</span>
                        <p class="text-xs text-slate-500 mt-0.5">比对数据差异并生成 SQL</p>
                    </div>
                </button>
            </div>
        </div>
    `;
}

async function renderDBConfig(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-8 flex items-center justify-between">
            <div>
            <h1 class="text-xl font-bold text-slate-900">数据库配置</h1>
                <p class="text-sm text-slate-500 mt-1">管理数据库连接配置</p>
            </div>
            <button onclick="showAddConfigModal()" class="btn btn-primary">
                <i class="fa fa-plus"></i>添加配置
            </button>
        </div>
        <div id="configList" class="space-y-3"></div>
        <div id="configModal"></div>
    `;
    renderConfigList();
}

async function loadDBConfigs() {
    try {
        const response = await fetch('/api/dbconfig', { headers: { 'Authorization': `Bearer ${token}` } });
        dbConfigs = await response.json();
    } catch { dbConfigs = []; }
}

function renderConfigList() {
    const container = document.getElementById('configList');
    if (!container) return;
    if (dbConfigs.length === 0) {
        container.innerHTML = `<div class="card p-10 text-center"><i class="fa fa-database text-3xl text-slate-300 mb-3"></i><p class="text-slate-500 text-sm">暂无数据库配置，请点击右上角添加</p></div>`;
        return;
    }
    container.innerHTML = dbConfigs.map(config => `
        <div class="card p-4 flex items-center justify-between card-hover">
            <div class="flex items-center gap-4">
                <div class="w-10 h-10 rounded-xl bg-gradient-to-br ${config.db_type === 'mysql' ? 'from-amber-500 to-amber-600' : config.db_type === 'postgres' ? 'from-blue-500 to-indigo-600' : 'from-slate-500 to-slate-600'} flex items-center justify-center shadow-sm">
                    <i class="fa ${config.db_type === 'mysql' ? 'fa-database' : config.db_type === 'postgres' ? 'fa-database' : 'fa-archive'} text-white text-sm"></i>
                </div>
                <div>
                    <h3 class="font-medium text-slate-900">${config.name}</h3>
                    <p class="text-xs text-slate-500 mt-0.5">
                        <span class="badge badge-blue">${config.db_type.toUpperCase()}</span>
                        <span class="ml-2">${config.host}:${config.port}/${config.db_name}</span>
                    </p>
                </div>
            </div>
            <div class="flex gap-2">
                <button onclick="testConnection(${config.id})" class="btn btn-ghost btn-sm">
                    <i class="fa fa-plug"></i>测试
                </button>
                <button onclick="deleteConfig(${config.id})" class="btn btn-ghost btn-sm text-red-500 hover:bg-red-50 hover:text-red-600">
                    <i class="fa fa-trash"></i>删除
                </button>
            </div>
        </div>
    `).join('');
}

function showAddConfigModal() {
    const modal = document.getElementById('configModal');
    modal.innerHTML = `
        <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 modal-overlay" onclick="if(event.target===this)document.querySelector('.fixed').remove()">
            <div class="modal-content bg-white rounded-2xl shadow-2xl p-6 w-full max-w-lg mx-4">
                <div class="flex items-center justify-between mb-6">
                    <h3 class="text-lg font-semibold text-slate-900">添加数据库配置</h3>
                    <button onclick="document.querySelector('.fixed').remove()" class="btn btn-ghost btn-sm p-1">
                        <i class="fa fa-times text-slate-400"></i>
                    </button>
                </div>
                <form id="configForm" class="space-y-4">
                    <div>
                        <label class="label">名称</label>
                        <input type="text" name="name" required class="input" placeholder="例如: 生产环境">
                    </div>
                    <div>
                        <label class="label">数据库类型</label>
                        <select name="db_type" required class="input">
                            <option value="mysql">MySQL</option>
                            <option value="postgres">PostgreSQL</option>
                            <option value="sqlite3">SQLite</option>
                        </select>
                    </div>
                    <div class="grid grid-cols-2 gap-4">
                        <div>
                            <label class="label">主机</label>
                            <input type="text" name="host" class="input" placeholder="localhost">
                        </div>
                        <div>
                            <label class="label">端口</label>
                            <input type="number" name="port" class="input" placeholder="3306">
                        </div>
                    </div>
                    <div>
                        <label class="label">数据库名</label>
                        <input type="text" name="db_name" class="input" placeholder="mydb">
                    </div>
                    <div class="grid grid-cols-2 gap-4">
                        <div>
                            <label class="label">用户名</label>
                            <input type="text" name="username" class="input" placeholder="root">
                        </div>
                        <div>
                            <label class="label">密码</label>
                            <input type="password" name="password" class="input" placeholder="••••••">
                        </div>
                    </div>
                    <div class="flex justify-end gap-3 pt-2">
                        <button type="button" onclick="document.querySelector('.fixed').remove()" class="btn btn-outline">取消</button>
                        <button type="submit" class="btn btn-primary">保存</button>
                    </div>
                </form>
            </div>
        </div>
    `;
    document.getElementById('configForm').addEventListener('submit', handleCreateConfig);
}

async function handleCreateConfig(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const config = {};
    for (const [key, value] of formData.entries()) config[key] = value;
    try {
        const response = await fetch('/api/dbconfig', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify(config)
        });
        if (response.ok) {
            document.querySelector('.fixed').remove();
            await loadDBConfigs();
            renderConfigList();
        } else {
            const result = await response.json();
            alert('创建失败: ' + (result.error || '未知错误'));
        }
    } catch (err) { alert('创建失败: ' + err.message); }
}

async function testConnection(configId) {
    try {
        const response = await fetch('/api/datasync/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ config_id: configId })
        });
        const result = await response.json();
        if (response.ok && result.success) alert('连接成功！');
        else alert('连接失败: ' + (result.error || '未知错误'));
    } catch (err) { alert('连接失败: ' + err.message); }
}

async function deleteConfig(configId) {
    if (!confirm('确定要删除这个配置吗？')) return;
    try {
        const response = await fetch(`/api/dbconfig/${configId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (response.ok) { await loadDBConfigs(); renderConfigList(); }
        else alert('删除失败');
    } catch (err) { alert('删除失败: ' + err.message); }
}

function updateTargetOptions(sourceSelectId, targetSelectId) {
    const sourceSelect = document.getElementById(sourceSelectId);
    const targetSelect = document.getElementById(targetSelectId);
    if (!sourceSelect || !targetSelect) return;
    const sourceId = sourceSelect.value;
    const selectedTargets = Array.from(targetSelect.selectedOptions).map(o => o.value);
    targetSelect.innerHTML = dbConfigs
        .filter(c => String(c.id) !== sourceId)
        .map(c => `<option value="${c.id}" ${selectedTargets.includes(String(c.id)) ? 'selected' : ''}>${c.name}</option>`)
        .join('');
}

async function renderSchemaCompare(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-5 flex items-start justify-between gap-4">
            <h1 class="text-xl font-bold text-slate-900">表结构比对</h1>
            <button id="schemaCompareBtn" onclick="compareSchemas()" class="btn btn-primary btn-sm"><i class="fa fa-refresh"></i>重新比对</button>
        </div>
        <p class="text-sm text-slate-500 -mt-4 mb-5">对比两个数据源中表结构的差异</p>
        <div class="card p-4 mb-5">
            <div class="schema-controls">
                <div>
                    <label class="label">源库</label>
                    <select id="schemaSourceConfig" class="input" onchange="renderSchemaTargetOptions(); loadSchemaTableOptions()">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div class="schema-switch" title="源库到目标库"><i class="fa fa-exchange"></i></div>
                <div>
                    <label class="label">目标库 <span class="text-xs text-slate-400 font-normal">(可多选)</span></label>
                    <div id="schemaTargetConfigs" class="schema-target-list"></div>
                </div>
            </div>
            <div class="schema-controls mt-3">
                <div>
                    <label class="label">表</label>
                    <details id="schemaTableDropdown" class="schema-table-dropdown">
                        <summary class="input cursor-pointer list-none"><span id="schemaTableSelectionLabel">全部表</span><i class="fa fa-chevron-down float-right mt-0.5 text-slate-400"></i></summary>
                        <div id="schemaTableOptions" class="schema-table-options"><span class="text-xs text-slate-400">请先选择源库</span></div>
                    </details>
                </div>
                <div class="schema-switch" title="表映射"><i class="fa fa-link"></i></div>
                <div>
                    <label class="label">目标表</label>
                    <input id="schemaTargetTable" class="input" value="全部表" readonly>
                </div>
            </div>
        </div>
        <div id="schemaCompareResult"></div>
    `;
    renderSchemaTargetOptions();
}

function renderSchemaTargetOptions() {
    const sourceID = document.getElementById('schemaSourceConfig')?.value;
    const container = document.getElementById('schemaTargetConfigs');
    if (!container) return;
    container.innerHTML = dbConfigs.filter(c => String(c.id) !== sourceID).map(c => `
        <label class="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-slate-50 cursor-pointer text-sm text-slate-700">
            <input type="checkbox" value="${c.id}" class="schema-target-option checkbox-custom">
            <span>${escapeHtml(c.name)}</span><span class="text-xs text-slate-400 ml-auto">${escapeHtml(c.db_type || '')}</span>
        </label>
    `).join('') || '<p class="text-sm text-slate-400 py-2">请先选择源数据库</p>';
}

async function loadSchemaTableOptions() {
    const sourceID = parseInt(document.getElementById('schemaSourceConfig')?.value);
    const options = document.getElementById('schemaTableOptions');
    if (!options) return;
    if (!sourceID) {
        schemaSourceTables = [];
        options.innerHTML = '<span class="text-xs text-slate-400">请先选择源库</span>';
        updateSchemaTableSelection();
        return;
    }
    options.innerHTML = '<span class="text-xs text-slate-400">正在加载表...</span>';
    try {
        const response = await fetch(`/api/schema/tables/${sourceID}`, { headers: { 'Authorization': `Bearer ${token}` } });
        if (!response.ok) throw new Error('加载表失败');
        schemaSourceTables = await response.json();
        options.innerHTML = `<label class="schema-table-option font-medium"><input type="checkbox" id="schemaAllTables" checked onchange="toggleAllSchemaTables(this.checked)">全部表</label>${schemaSourceTables.map(table => `<label class="schema-table-option"><input type="checkbox" value="${escapeHtml(table)}" checked onchange="updateSchemaTableSelection()">${escapeHtml(table)}</label>`).join('')}`;
        updateSchemaTableSelection();
    } catch (err) {
        schemaSourceTables = [];
        options.innerHTML = '<span class="text-xs text-red-500">表加载失败</span>';
        updateSchemaTableSelection();
    }
}

function toggleAllSchemaTables(checked) {
    document.querySelectorAll('#schemaTableOptions input[type="checkbox"]:not(#schemaAllTables)').forEach(input => { input.checked = checked; });
    updateSchemaTableSelection();
}

function updateSchemaTableSelection() {
    const all = document.getElementById('schemaAllTables');
    const tableInputs = Array.from(document.querySelectorAll('#schemaTableOptions input[type="checkbox"]:not(#schemaAllTables)'));
    const selected = tableInputs.filter(input => input.checked);
    if (all && tableInputs.length > 0 && selected.length === tableInputs.length) all.checked = true;
    if (all && selected.length < tableInputs.length) all.checked = false;
    const label = document.getElementById('schemaTableSelectionLabel');
    const target = document.getElementById('schemaTargetTable');
    const text = !tableInputs.length || selected.length === tableInputs.length ? '全部表' : selected.length === 0 ? '未选择表' : `已选择 ${selected.length} 张表`;
    if (label) label.textContent = text;
    if (target) target.value = text;
}

function getSelectedSchemaTables() {
    const tableInputs = Array.from(document.querySelectorAll('#schemaTableOptions input[type="checkbox"]:not(#schemaAllTables)'));
    const selected = tableInputs.filter(input => input.checked).map(input => input.value);
    return !tableInputs.length || selected.length === tableInputs.length ? [] : selected;
}

async function compareSchemas() {
    const sourceID = parseInt(document.getElementById('schemaSourceConfig').value);
    const targetIDs = Array.from(document.querySelectorAll('#schemaTargetConfigs input:checked')).map(o => parseInt(o.value));
    const tables = getSelectedSchemaTables();
    if (!sourceID || targetIDs.length === 0 || (document.getElementById('schemaTargetTable')?.value === '未选择表')) {
        alert('请选择源数据库和至少一个目标数据库');
        return;
    }
    const resultDiv = document.getElementById('schemaCompareResult');
    const compareButton = document.getElementById('schemaCompareBtn');
    compareButton.disabled = true;
    compareButton.innerHTML = '<i class="fa fa-spinner fa-spin"></i>正在比对...';
    resultDiv.innerHTML = `
        <div class="card p-6 mb-6">
            <div id="schemaCompareProgress" class="flex items-center gap-2 text-blue-600 font-medium text-sm">
                <i class="fa fa-spinner fa-spin"></i>正在比对表结构并生成 SQL
            </div>
            <pre id="schemaCompareLogs" class="mt-4 max-h-72 overflow-auto log-terminal bg-slate-900 text-slate-100 p-4 rounded-lg"></pre>
        </div>
        <section id="schemaCompareResultPanel" class="card p-6 hidden">
            <h2 class="text-base font-semibold text-slate-900 mb-4">比对结果</h2>
            <div id="schemaCompareFinalResult"></div>
        </section>
    `;
    resetCompareLogs('schema');
    try {
        const response = await fetch('/api/schema/compare/stream', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ source_id: sourceID, target_ids: targetIDs, tables })
        });
        if (!response.ok) { const errorResult = await response.json(); throw new Error(errorResult.error || '比对失败'); }
        const results = await readSchemaCompareStream(response);
        document.getElementById('schemaCompareProgress')?.closest('.card')?.remove();
        renderSchemaCompareResults(results, document.getElementById('schemaCompareFinalResult'));
    } catch (err) {
        const logContainer = document.getElementById('schemaCompareLogs');
        if (logContainer) appendSchemaCompareLog({ level: 'error', message: `比对失败: ${err.message}` });
        else resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">比对失败: ${err.message}</div>`;
    } finally {
        compareButton.disabled = false;
        compareButton.innerHTML = '<i class="fa fa-refresh"></i>重新比对';
    }
}

async function readSchemaCompareStream(response) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '', results = null;
    while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split('\n\n');
        buffer = events.pop();
        const eventResult = processSchemaCompareEvents(events, results);
        results = eventResult.results;
        if (eventResult.completed) {
            await reader.cancel();
            return results;
        }
    }
    if (buffer.trim()) {
        const eventResult = processSchemaCompareEvents([buffer], results);
        results = eventResult.results;
        if (eventResult.completed) return results;
    }
    if (!results) throw new Error('未收到比对结果');
    return results;
}

function processSchemaCompareEvents(events, currentResults) {
    let results = currentResults;
    let completed = false;
    events.forEach(rawEvent => {
        const parsedEvent = parseSSEEvent(rawEvent);
        if (!parsedEvent) return;
        if (parsedEvent.name === 'log') { appendSchemaCompareLog(parsedEvent.data); return; }
        if (parsedEvent.name === 'error') throw new Error(parsedEvent.data.message || '比对失败');
        if (parsedEvent.name === 'complete') { results = parsedEvent.data.results; completed = parsedEvent.data.completed === true; }
    });
    return { results, completed };
}

function appendSchemaCompareLog(log) {
    enqueueCompareLog('schema', log);
}

function renderSchemaCompareResults(results, container) {
    document.getElementById('schemaCompareResultPanel')?.classList.remove('hidden');
    if (!results || results.length === 0) {
        container.innerHTML = '<div class="text-center py-8 text-slate-500 text-sm">没有比对结果</div>';
        return;
    }
    window.currentSchemaResults = results;
    container.innerHTML = results.map((result, index) => renderSchemaCompareWorkspace(result, index)).join('');
    if (window.hljs) container.querySelectorAll('code.language-sql').forEach(code => window.hljs.highlightElement(code));
}

function renderSchemaCompareWorkspace(result, resultIndex) {
    if (result.error) return `<div class="schema-workspace p-4 text-sm text-red-600">${escapeHtml(result.target_name)}：${escapeHtml(result.error)}</div>`;
    const tableDiffs = result.diff?.table_diffs || [];
    return `
        <div class="schema-workspace">
            <div class="schema-workspace-header">
                <div class="flex items-center gap-3 min-w-0">
                    <span class="font-semibold text-slate-800 truncate">目标库：${escapeHtml(result.target_name)}</span>
                    <span class="text-xs text-slate-400">${(result.sql || []).length} 条 SQL</span>
                    ${tableDiffs.length ? `<select class="input w-40 py-1.5 text-xs" onchange="selectSchemaResultTable(${resultIndex}, this.value)"><option value="">全部变更</option>${tableDiffs.map(diff => `<option value="${escapeHtml(diff.table_name)}">${escapeHtml(diff.table_name)}</option>`).join('')}</select>` : ''}
                </div>
                <div class="flex gap-2 flex-shrink-0">
                    <button onclick="downloadSchemaSQL(${resultIndex})" class="btn btn-outline btn-sm"><i class="fa fa-download"></i>下载 .sql</button>
                    <button onclick="showSchemaSQLDialog(${resultIndex})" class="btn btn-primary btn-sm"><i class="fa fa-code"></i>查看 SQL</button>
                </div>
            </div>
            <div id="schemaResultDetail_${resultIndex}">${renderSchemaResultDetail(result, '')}</div>
        </div>`;
}

function selectSchemaResultTable(resultIndex, tableName) {
    const result = window.currentSchemaResults?.[resultIndex];
    const container = document.getElementById(`schemaResultDetail_${resultIndex}`);
    if (result && container) {
        container.innerHTML = renderSchemaResultDetail(result, tableName);
        if (window.hljs) container.querySelectorAll('code.language-sql').forEach(code => window.hljs.highlightElement(code));
    }
}

function renderSchemaResultDetail(result, tableName) {
    const allTableDiffs = result.diff?.table_diffs || [];
    const tableDiffs = tableName ? allTableDiffs.filter(diff => diff.table_name === tableName) : allTableDiffs;
    if (!tableDiffs.length) return '<div class="p-6 text-sm text-slate-500">表结构一致，无差异</div>';
    const groups = getSchemaChangeGroups(tableDiffs);
    return `
        <div class="schema-result-layout">
            <aside class="schema-diff-summary">
                <div class="px-3 py-3 text-sm font-semibold text-slate-700 border-b border-slate-200">变更详情</div>
                ${groups.map(group => `<section class="schema-diff-group">
                    <div class="flex items-center justify-between text-xs font-semibold ${group.color}"><span><i class="fa ${group.icon} mr-1"></i>${group.label}</span><span>${group.items.length}</span></div>
                    <div class="mt-1">${group.items.map(item => `<div class="schema-diff-item"><span class="schema-diff-dot ${group.dot}"></span><span><strong class="font-medium text-slate-700">${escapeHtml(item.name)}</strong><small class="block text-slate-400 truncate">${escapeHtml(item.detail)}</small></span></div>`).join('')}</div>
                </section>`).join('') || '<div class="p-3 text-xs text-slate-400">没有结构差异</div>'}
            </aside>
            <div class="schema-main">
                <div class="flex items-center justify-between mb-2"><h3 class="text-sm font-semibold text-slate-700">生成的 SQL</h3><span class="text-xs text-slate-400">在目标库执行以保持一致</span></div>
                ${(result.sql || []).length ? `<pre class="schema-sql"><code class="language-sql">${escapeHtml(result.sql.join('\n\n'))}</code></pre>` : '<div class="schema-sql p-4 text-xs text-slate-400">无需执行 SQL</div>'}
                <div class="text-xs font-semibold text-slate-500 mt-4 mb-2">表结构预览</div>
                ${tableDiffs.map(tableDiff => renderSchemaTablePreviewPair(tableDiff)).join('')}
            </div>
        </div>`;
}

function getSchemaChangeGroups(tableDiffs) {
    const groups = [
        { label: '新增', icon: 'fa-plus', color: 'text-emerald-700', dot: 'bg-emerald-500', items: [] },
        { label: '修改', icon: 'fa-pencil', color: 'text-amber-700', dot: 'bg-amber-500', items: [] },
        { label: '删除', icon: 'fa-trash', color: 'text-red-700', dot: 'bg-red-500', items: [] }
    ];
    const groupIndex = { CREATE: 0, ALTER: 1, DROP: 2 };
    tableDiffs.forEach(tableDiff => {
        if (tableDiff.type !== 'ALTER') groups[groupIndex[tableDiff.type]].items.push({ name: tableDiff.table_name, detail: tableDiff.type === 'CREATE' ? '新增数据表' : '删除数据表' });
        (tableDiff.column_diffs || []).forEach(diff => {
            const column = diff.source_column || diff.target_column || {};
            const detail = diff.type === 'ALTER' ? `${diff.target_column?.type || ''} -> ${diff.source_column?.type || ''}` : column.type || '字段变更';
            groups[groupIndex[diff.type]].items.push({ name: `${tableDiff.table_name}.${diff.column_name}`, detail });
        });
        if (tableDiff.table_comment_changed) groups[1].items.push({ name: tableDiff.table_name, detail: '表备注已修改' });
    });
    return groups.filter(group => group.items.length > 0);
}

function renderSchemaTablePreviewPair(tableDiff) {
    return `<div class="schema-preview-grid"><div class="col-span-full text-xs text-slate-400 -mb-2">${escapeHtml(tableDiff.table_name)}</div>${renderSchemaTablePreview(tableDiff.source_table, '源库', tableDiff, 'source')}<i class="fa fa-long-arrow-right text-slate-400 text-center"></i>${renderSchemaTablePreview(tableDiff.target_table, '目标库', tableDiff, 'target')}</div>`;
}

function renderSchemaTablePreview(table, title, tableDiff, side) {
    const columns = table?.columns || [];
    return `<div class="schema-preview"><div class="schema-preview-title">${title} ${escapeHtml(table?.name || tableDiff.table_name)}</div><table><thead><tr><th>字段名</th><th>类型</th><th>可空</th><th>注释</th></tr></thead><tbody>${columns.map(column => `<tr class="${getSchemaPreviewChangeClass(tableDiff, column.name, side)}"><td>${escapeHtml(column.name)}</td><td>${escapeHtml(column.type)}</td><td>${column.nullable ? '是' : '否'}</td><td>${escapeHtml(column.comment || '-')}</td></tr>`).join('') || '<tr><td colspan="4" class="text-slate-400">无字段</td></tr>'}</tbody></table></div>`;
}

function getSchemaPreviewChangeClass(tableDiff, columnName, side) {
    if (tableDiff.type === 'CREATE') return side === 'source' ? 'schema-preview-create' : '';
    if (tableDiff.type === 'DROP') return side === 'target' ? 'schema-preview-drop' : '';
    const columnDiff = (tableDiff.column_diffs || []).find(diff => diff.column_name === columnName);
    if (!columnDiff) return '';
    if (columnDiff.type === 'ALTER') return 'schema-preview-alter';
    if (columnDiff.type === 'CREATE') return side === 'source' ? 'schema-preview-create' : '';
    return side === 'target' ? 'schema-preview-drop' : '';
}

function showSchemaSQLDialog(resultIndex) {
    const result = window.currentSchemaResults[resultIndex];
    if (!result || !result.sql || result.sql.length === 0) { alert('没有生成SQL'); return; }
    window.currentSchemaDialogIndex = resultIndex;
    const sqlContent = result.sql.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/50 flex items-center justify-center z-50 modal-overlay';
    modal.innerHTML = `
        <div class="modal-content bg-white rounded-2xl shadow-2xl w-full max-w-5xl max-h-[85vh] overflow-hidden mx-4">
            <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200">
                <h3 class="text-lg font-semibold text-slate-900">表结构比对 - ${result.target_name}</h3>
                <div class="flex gap-2">
                    <button id="schemaDownloadBtn" class="btn btn-primary btn-sm"><i class="fa fa-download"></i>下载 SQL</button>
                    <button id="schemaExecuteBtn" class="btn btn-success btn-sm"><i class="fa fa-play"></i>执行 SQL</button>
                    <button id="schemaCancelBtn" class="btn btn-ghost btn-sm"><i class="fa fa-times"></i>取消</button>
                </div>
            </div>
            <div class="p-6 overflow-auto max-h-[60vh] bg-slate-50">
                <pre class="rounded-lg overflow-x-auto text-sm"><code id="schemaSqlCode" class="language-sql" style="background: transparent;">${escapeHtml(sqlContent)}</code></pre>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    if (window.hljs) { const el = document.getElementById('schemaSqlCode'); el.textContent = sqlContent; window.hljs.highlightElement(el); }
    document.getElementById('schemaCancelBtn').addEventListener('click', () => modal.remove());
    document.getElementById('schemaDownloadBtn').addEventListener('click', () => {
        const res = window.currentSchemaResults[window.currentSchemaDialogIndex];
        downloadSchemaSQLFile(res.target_name, res.sql);
    });
    document.getElementById('schemaExecuteBtn').addEventListener('click', () => executeSchemaSQL(window.currentSchemaDialogIndex, modal));
}

async function executeSchemaSQL(resultIndex, modal) {
    const result = window.currentSchemaResults[resultIndex];
    const sourceID = parseInt(document.getElementById('schemaSourceConfig').value);
    if (!confirm(`确定要执行SQL到目标数据库 ${result.target_name} 吗？此操作不可撤销！`)) return;
    modal.remove();
    try {
        const response = await fetch('/api/schema/execute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ source_id: sourceID, target_id: result.target_id, sql: result.sql })
        });
        const data = await response.json();
        if (response.ok) {
            alert(`执行完成: ${data.executed}/${data.total} 条成功\n状态: ${data.status}\n${data.errors.length > 0 ? '错误: ' + data.errors.join('\n') : ''}`);
        } else {
            alert('执行失败: ' + (data.error || '未知错误'));
        }
    } catch (err) { alert('执行失败: ' + err.message); }
}

function downloadSchemaSQL(resultIndex) {
    const result = window.currentSchemaResults[resultIndex];
    if (!result || !result.sql || result.sql.length === 0) { alert('没有SQL可下载'); return; }
    downloadSchemaSQLFile(result.target_name, result.sql);
}

function downloadSchemaSQLFile(targetName, sqlArray) {
    const blob = new Blob([sqlArray.join('\n\n')], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = `${targetName}_schema_compare.sql`;
    document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url);
}


async function renderDataCompare(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-8">
            <h1 class="text-xl font-bold text-slate-900">数据对比</h1>
            <p class="text-sm text-slate-500 mt-1">按选定字段匹配记录，忽略 id 后生成差异 SQL</p>
        </div>
        <div class="card p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
                <div>
                    <label class="label">源数据库</label>
                    <select id="dataSourceConfig" class="input">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="label">目标数据库 <span class="text-xs text-slate-400 font-normal">(可多选)</span></label>
                    <select id="dataTargetConfigs" multiple class="input h-36">
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="label">比对表</label>
                    <input id="dataTable" class="input" list="dataTableOptions" disabled onchange="loadComparisonFields()" placeholder="搜索或选择比对表">
                    <datalist id="dataTableOptions"></datalist>
                    <p id="tableCountInfo" class="mt-1.5 text-xs text-slate-400 hidden"></p>
                </div>
                <div>
                    <label class="label">比对字段</label>
                    <input id="comparisonField" class="input" list="comparisonFieldOptions" disabled placeholder="搜索或选择比对字段">
                    <datalist id="comparisonFieldOptions"></datalist>
                    <p class="mt-1.5 text-xs text-slate-400">使用字段值匹配数据行，id 不参与比对或 SQL</p>
                </div>
            </div>
            <div class="mt-5 flex items-center gap-2">
                <input type="checkbox" id="deleteMissing" class="checkbox-custom">
                <label for="deleteMissing" class="text-sm text-slate-600 cursor-pointer">删除目标数据库中不存在于源数据库的数据</label>
            </div>
            <div class="mt-5">
                <button id="dataCompareBtn" onclick="compareData()" class="btn btn-success btn-lg">
                    <i class="fa fa-search"></i>比对并生成 SQL
                </button>
            </div>
        </div>
        <div id="dataCompareResult"></div>
    `;
    document.getElementById('dataSourceConfig').addEventListener('change', () => {
        updateTargetOptions('dataSourceConfig', 'dataTargetConfigs');
        loadSourceTables();
    });
}

async function loadSourceTables() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const tableSelect = document.getElementById('dataTable');
    const fieldSelect = document.getElementById('comparisonField');
    const countInfo = document.getElementById('tableCountInfo');
    if (!sourceID) {
        tableSelect.value = '';
        document.getElementById('dataTableOptions').replaceChildren();
        fieldSelect.value = '';
        document.getElementById('comparisonFieldOptions').replaceChildren();
        tableSelect.disabled = true;
        fieldSelect.disabled = true;
        countInfo.classList.add('hidden');
        return;
    }
    tableSelect.disabled = true;
    tableSelect.value = '';
    document.getElementById('dataTableOptions').innerHTML = '<option value="正在加载表..."></option>';
    fieldSelect.value = '';
    document.getElementById('comparisonFieldOptions').replaceChildren();
    fieldSelect.disabled = true;
    try {
        const response = await fetch(`/api/schema/tables/${sourceID}`, { headers: { 'Authorization': `Bearer ${token}` } });
        if (!response.ok) throw new Error('加载表失败');
        const tables = await response.json();
        renderTableOptions(tables);
        tableSelect.disabled = false;
        if (tables.length > 0) { countInfo.textContent = `共 ${tables.length} 张表`; countInfo.classList.remove('hidden'); }
        else countInfo.classList.add('hidden');
    } catch (err) { tableSelect.value = ''; document.getElementById('dataTableOptions').innerHTML = '<option value="加载失败"></option>'; countInfo.classList.add('hidden'); }
}

function renderTableOptions(tables) {
    const tableSelect = document.getElementById('dataTable');
    document.getElementById('dataTableOptions').innerHTML = tables.map(t => `<option value="${escapeHtml(t)}"></option>`).join('');
    tableSelect.placeholder = tables.length ? '搜索或选择比对表' : '没有可用表';
}

async function loadComparisonFields() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const tableName = document.getElementById('dataTable').value;
    const fieldSelect = document.getElementById('comparisonField');
    if (!sourceID || !tableName) {
        fieldSelect.value = '';
        document.getElementById('comparisonFieldOptions').replaceChildren();
        fieldSelect.disabled = true;
        return;
    }
    fieldSelect.disabled = true;
    fieldSelect.value = '';
    document.getElementById('comparisonFieldOptions').innerHTML = '<option value="正在加载字段..."></option>';
    try {
        const response = await fetch(`/api/datasync/columns/${sourceID}/${encodeURIComponent(tableName)}`, { headers: { 'Authorization': `Bearer ${token}` } });
        if (!response.ok) throw new Error('加载字段失败');
        const columns = await response.json();
        document.getElementById('comparisonFieldOptions').innerHTML = columns.map(column => `<option value="${escapeHtml(column)}"></option>`).join('');
        fieldSelect.placeholder = columns.length ? '搜索或选择比对字段' : '没有可用字段';
        fieldSelect.disabled = columns.length === 0;
    } catch (err) { fieldSelect.value = ''; document.getElementById('comparisonFieldOptions').innerHTML = '<option value="加载失败"></option>'; }
}

async function compareData() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const targetIDs = Array.from(document.getElementById('dataTargetConfigs').selectedOptions).map(o => parseInt(o.value));
    const tableName = document.getElementById('dataTable').value;
    const comparisonField = document.getElementById('comparisonField').value;
    const deleteMissing = document.getElementById('deleteMissing').checked;
    if (!sourceID || targetIDs.length === 0 || !tableName || !comparisonField) {
        alert('请选择源数据库、至少一个目标数据库、比对表和比对字段');
        return;
    }
    const resultDiv = document.getElementById('dataCompareResult');
    const compareButton = document.getElementById('dataCompareBtn');
    compareButton.disabled = true;
    compareButton.innerHTML = '<i class="fa fa-spinner fa-spin"></i>正在比对...';
    resultDiv.innerHTML = `
        <div class="card p-6 mb-6">
            <div id="dataCompareProgress" class="flex items-center gap-2 text-blue-600 font-medium text-sm">
                <i class="fa fa-spinner fa-spin"></i>正在比对数据并生成 SQL
            </div>
            <pre id="dataCompareLogs" class="mt-4 max-h-72 overflow-auto log-terminal bg-slate-900 text-slate-100 p-4 rounded-lg"></pre>
        </div>
        <section class="card p-6">
            <h2 class="text-base font-semibold text-slate-900 mb-4">比对结果</h2>
            <div id="dataCompareFinalResult"></div>
        </section>
    `;
    resetCompareLogs('data');
    try {
        const response = await fetch('/api/datasync/sync/stream', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ source_id: sourceID, target_ids: targetIDs, tables: [tableName], key_fields: { [tableName]: comparisonField }, delete_missing: deleteMissing })
        });
        if (!response.ok) { const errorResult = await response.json(); throw new Error(errorResult.error || '比对失败'); }
        const results = await readDataCompareStream(response);
        markCompareCompleted('dataCompareProgress');
        renderDataCompareResults(results, document.getElementById('dataCompareFinalResult'));
    } catch (err) {
        const logContainer = document.getElementById('dataCompareLogs');
        if (logContainer) appendDataCompareLog({ level: 'error', message: `生成SQL失败: ${err.message}` });
        else resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">生成SQL失败: ${err.message}</div>`;
    } finally {
        compareButton.disabled = false;
        compareButton.innerHTML = '<i class="fa fa-search"></i>比对并生成 SQL';
    }
}

async function readDataCompareStream(response) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '', results = null;
    while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split('\n\n');
        buffer = events.pop();
        const eventResult = processDataCompareEvents(events, results);
        results = eventResult.results;
        if (eventResult.completed) {
            await reader.cancel();
            return results;
        }
    }
    if (buffer.trim()) {
        const eventResult = processDataCompareEvents([buffer], results);
        results = eventResult.results;
        if (eventResult.completed) return results;
    }
    if (!results) throw new Error('未收到比对结果');
    return results;
}

function processDataCompareEvents(events, currentResults) {
    let results = currentResults;
    let completed = false;
    events.forEach(rawEvent => {
        const parsedEvent = parseSSEEvent(rawEvent);
        if (!parsedEvent) return;
        if (parsedEvent.name === 'log') { appendDataCompareLog(parsedEvent.data); return; }
        if (parsedEvent.name === 'error') throw new Error(parsedEvent.data.message || '比对失败');
        if (parsedEvent.name === 'complete') { results = parsedEvent.data.results; completed = parsedEvent.data.completed === true; }
    });
    return { results, completed };
}

function markCompareCompleted(progressID) {
    const progress = document.getElementById(progressID);
    if (!progress) return;
    progress.className = 'flex items-center gap-2 text-emerald-600 font-medium text-sm';
    progress.innerHTML = '<i class="fa fa-check-circle"></i>比对完成';
}

function parseSSEEvent(rawEvent) {
    let eventName = 'message';
    const dataLines = [];
    rawEvent.split('\n').forEach(line => {
        if (line.startsWith('event:')) eventName = line.slice('event:'.length).trim();
        if (line.startsWith('data:')) dataLines.push(line.slice('data:'.length).trim());
    });
    if (dataLines.length === 0) return null;
    try { return { name: eventName, data: JSON.parse(dataLines.join('\n')) }; } catch { return null; }
}

function appendDataCompareLog(log) {
    enqueueCompareLog('data', log);
}

function resetCompareLogs(type) {
    const state = compareLogState[type];
    state.entries = [];
    const container = document.getElementById(state.containerID);
    if (container) container.replaceChildren();
}

function enqueueCompareLog(type, log) {
    const state = compareLogState[type];
    state.entries.push({ ...log, timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }) });
    if (state.entries.length > COMPARE_LOG_MAX_SIZE) state.entries.shift();
    if (state.renderScheduled) return;
    state.renderScheduled = true;
    requestAnimationFrame(() => renderCompareLogs(type));
}

function renderCompareLogs(type) {
    const state = compareLogState[type];
    state.renderScheduled = false;
    const container = document.getElementById(state.containerID);
    if (!container) return;
    const fragment = document.createDocumentFragment();
    state.entries.forEach(log => {
        const line = document.createElement('div');
        const timestamp = document.createElement('span');
        timestamp.className = log.level === 'error' ? 'text-red-400' : log.level === 'warn' ? 'text-yellow-400' : 'text-slate-300';
        timestamp.textContent = `[${log.timestamp}] `;
        line.append(timestamp);
        if (log.progress > 0) {
            const progress = document.createElement('span');
            progress.className = 'text-blue-400';
            progress.textContent = `${log.progress.toFixed(1)}% `;
            line.append(progress);
        }
        const context = [log.target_name ? `目标库: ${log.target_name}` : '', log.table_name ? `表: ${log.table_name}` : ''].filter(Boolean).join(' | ');
        if (context) {
            const contextNode = document.createElement('span');
            contextNode.className = 'text-blue-400';
            contextNode.textContent = `${context} | `;
            line.append(contextNode);
        }
        line.append(document.createTextNode(log.message || ''));
        fragment.append(line);
    });
    container.replaceChildren(fragment);
    container.scrollTop = container.scrollHeight;
}

function renderDataCompareResults(results, container) {
    if (!results || results.length === 0) {
        container.innerHTML = `<div class="card p-10 text-center text-slate-500 text-sm">没有对比结果</div>`;
        return;
    }
    let html = '';
    results.forEach((result, index) => {
        const changeSummary = getDataChangeSummary(result.results);
        const allSQL = getAllDataSQL(result.results);
        html += `
            <div class="card p-6 mb-5">
                <div class="flex items-center gap-3 mb-5">
                    <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-blue-600 flex items-center justify-center">
                        <i class="fa fa-database text-white text-xs"></i>
                    </div>
                    <div>
                        <h3 class="font-semibold text-slate-900">${result.target_name}</h3>
                        <p class="text-xs text-slate-500">${allSQL.length} 条 SQL 待执行</p>
                    </div>
                </div>
                ${allSQL.length > 0 ? `
                    <div class="mb-4">
                        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.INSERT, changeSummary[DATA_CHANGE_TYPE.INSERT])}
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.UPDATE, changeSummary[DATA_CHANGE_TYPE.UPDATE])}
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.DELETE, changeSummary[DATA_CHANGE_TYPE.DELETE])}
                        </div>
                        <div class="mt-4 flex justify-end gap-2">
                            <button id="dataTypeDownloadBtn_${index}" onclick="downloadSelectedDataSQL(${index})" disabled class="btn btn-sm bg-slate-200 text-slate-400 cursor-not-allowed">
                                <i class="fa fa-download"></i>下载已选 SQL
                            </button>
                            <button onclick="showDataSQLDialog(${index})" class="btn btn-primary btn-sm">
                                <i class="fa fa-eye"></i>查看 SQL (${allSQL.length}条)
                            </button>
                        </div>
                    </div>
                ` : ''}
                ${result.error ? `<div class="badge badge-red">${result.error}</div>` : renderTableResults(result.results, index)}
            </div>
        `;
    });
    container.innerHTML = html;
    window.currentDataResults = results;
}

function renderDataChangeTypeOption(targetIndex, type, tableChanges) {
    const config = DATA_CHANGE_TYPE_CONFIG[type];
    const changeCount = tableChanges.reduce((count, change) => count + change.sql.length, 0);
    const tableList = tableChanges.length > 0
        ? tableChanges.map(change => `<li class="truncate">${escapeHtml(change.table)} <span class="text-slate-400">(${change.sql.length})</span></li>`).join('')
        : '<li class="text-slate-400">无变更</li>';
    const disabled = changeCount === 0;
    return `
        <label class="block border ${config.borderClass} ${config.backgroundClass} rounded-lg p-3 ${disabled ? 'opacity-50' : 'cursor-pointer'} transition">
            <div class="flex items-center justify-between gap-3">
                <span class="flex items-center gap-2 font-medium ${config.colorClass}">
                    <input type="checkbox" class="data-change-type-checkbox checkbox-custom" data-target-index="${targetIndex}" data-change-type="${type}" onchange="updateDataSQLDownloadState(${targetIndex})" ${disabled ? 'disabled' : ''}>
                    <i class="fa ${config.icon}"></i>${config.label}
                </span>
                <span class="text-sm font-medium ${config.colorClass}">${changeCount} 条</span>
            </div>
            <ul class="mt-2 text-xs text-slate-600 space-y-0.5">${tableList}</ul>
        </label>
    `;
}

function getDataChangeSummary(tableResults) {
    const summary = { [DATA_CHANGE_TYPE.INSERT]: [], [DATA_CHANGE_TYPE.UPDATE]: [], [DATA_CHANGE_TYPE.DELETE]: [] };
    (tableResults || []).forEach(tableResult => {
        const sqlByType = { [DATA_CHANGE_TYPE.INSERT]: [], [DATA_CHANGE_TYPE.UPDATE]: [], [DATA_CHANGE_TYPE.DELETE]: [] };
        (tableResult.sql || []).forEach(sql => { const type = getDataChangeType(sql); if (type) sqlByType[type].push(sql); });
        Object.keys(sqlByType).forEach(type => { if (sqlByType[type].length > 0) summary[type].push({ table: tableResult.table, sql: sqlByType[type] }); });
    });
    return summary;
}

function getDataChangeType(sql) {
    const normalizedSQL = sql.trim().toUpperCase();
    if (normalizedSQL.startsWith('INSERT INTO')) return DATA_CHANGE_TYPE.INSERT;
    if (normalizedSQL.startsWith('UPDATE')) return DATA_CHANGE_TYPE.UPDATE;
    if (normalizedSQL.startsWith('DELETE FROM')) return DATA_CHANGE_TYPE.DELETE;
    return null;
}

function getAllDataSQL(tableResults) { return (tableResults || []).flatMap(tableResult => tableResult.sql || []); }

function updateDataSQLDownloadState(targetIndex) {
    const selectedTypes = getSelectedDataChangeTypes(targetIndex);
    const button = document.getElementById(`dataTypeDownloadBtn_${targetIndex}`);
    if (!button) return;
    const enabled = selectedTypes.length > 0;
    button.disabled = !enabled;
    if (enabled) { button.className = 'btn btn-success btn-sm'; button.innerHTML = '<i class="fa fa-download"></i>下载已选 SQL'; }
    else { button.className = 'btn btn-sm bg-slate-200 text-slate-400 cursor-not-allowed'; button.innerHTML = '<i class="fa fa-download"></i>下载已选 SQL'; }
}

function getSelectedDataChangeTypes(targetIndex) {
    return Array.from(document.querySelectorAll(`.data-change-type-checkbox[data-target-index="${targetIndex}"]:checked`)).map(cb => cb.dataset.changeType);
}

function downloadSelectedDataSQL(targetIndex) {
    const result = window.currentDataResults[targetIndex];
    const selectedTypes = getSelectedDataChangeTypes(targetIndex);
    if (!result || selectedTypes.length === 0) return;
    const selectedTypeSet = new Set(selectedTypes);
    const sql = getAllDataSQL(result.results).filter(statement => selectedTypeSet.has(getDataChangeType(statement)));
    if (sql.length === 0) { alert('未找到所选类型的SQL'); return; }
    const typeLabel = selectedTypes.map(type => DATA_CHANGE_TYPE_CONFIG[type].label).join('_');
    downloadSQL(`${result.target_name}_${typeLabel}`, sql);
}

function renderTableResults(tableResults, targetIndex) {
    if (!tableResults || tableResults.length === 0) return `<div class="text-slate-500 text-sm text-center py-4">没有表数据</div>`;
    let html = '<div class="overflow-x-auto rounded-lg border border-slate-200">';
    html += '<table class="w-full table-modern">';
    html += `<thead><tr>
        <th class="text-left py-3 px-4">表名</th>
        <th class="text-center py-3 px-4">总数</th>
        <th class="text-center py-3 px-4">新增</th>
        <th class="text-center py-3 px-4">更新</th>
        <th class="text-center py-3 px-4">删除</th>
        <th class="text-center py-3 px-4">状态</th>
        <th class="text-center py-3 px-4">SQL</th>
    </tr></thead><tbody>`;
    tableResults.forEach((result, index) => {
        const hasError = result.error;
        const hasSQL = result.sql && result.sql.length > 0;
        html += `<tr class="border-b border-slate-100">
            <td class="py-3 px-4 font-medium text-slate-800">${result.table}</td>
            <td class="py-3 px-4 text-center text-slate-600">${result.total || 0}</td>
            <td class="py-3 px-4 text-center text-emerald-600 font-medium">${result.inserted || 0}</td>
            <td class="py-3 px-4 text-center text-blue-600 font-medium">${result.updated || 0}</td>
            <td class="py-3 px-4 text-center text-red-600 font-medium">${result.deleted || 0}</td>
            <td class="py-3 px-4 text-center">${hasError ? `<span class="badge badge-red">${result.error}</span>` : '<span class="badge badge-green">成功</span>'}</td>
            <td class="py-3 px-4 text-center">${hasSQL ? `<button onclick="showTableSQL(${targetIndex}, ${index})" class="btn btn-primary btn-sm"><i class="fa fa-eye"></i>查看</button>` : '-'}</td>
        </tr>`;
    });
    html += '</tbody></table></div>';
    return html;
}

function showDataSQLDialog(resultIndex) {
    const result = window.currentDataResults[resultIndex];
    if (!result || !result.results) { alert('没有数据结果'); return; }
    const allSQL = getAllDataSQL(result.results);
    if (allSQL.length === 0) { alert('没有生成SQL'); return; }
    window.currentDataDialogIndex = resultIndex;
    const sqlContent = allSQL.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/50 flex items-center justify-center z-50 modal-overlay';
    modal.innerHTML = `
        <div class="modal-content bg-white rounded-2xl shadow-2xl w-full max-w-5xl max-h-[85vh] overflow-hidden mx-4">
            <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200">
                <h3 class="text-lg font-semibold text-slate-900">数据对比 - ${result.target_name}</h3>
                <button id="dataCancelBtn" class="btn btn-ghost btn-sm"><i class="fa fa-times"></i>取消</button>
            </div>
            <div class="p-6 overflow-auto max-h-[60vh] bg-slate-50">
                <pre class="rounded-lg overflow-x-auto text-sm"><code id="dataSqlCode" class="language-sql" style="background: transparent;">${escapeHtml(sqlContent)}</code></pre>
            </div>
            <div class="flex justify-end gap-3 px-6 py-4 border-t border-slate-200 bg-slate-50">
                <button id="dataDownloadBtn" class="btn btn-primary"><i class="fa fa-download"></i>下载 SQL</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    if (window.hljs) { const el = document.getElementById('dataSqlCode'); el.textContent = sqlContent; window.hljs.highlightElement(el); }
    document.getElementById('dataCancelBtn').addEventListener('click', () => modal.remove());
    document.getElementById('dataDownloadBtn').addEventListener('click', () => {
        downloadSQL(window.currentDataResults[window.currentDataDialogIndex].target_name, getAllDataSQL(window.currentDataResults[window.currentDataDialogIndex].results));
    });
}

function showTableSQL(targetIndex, tableIndex) {
    const targetResult = window.currentDataResults[targetIndex];
    const result = targetResult && targetResult.results ? targetResult.results[tableIndex] : null;
    if (!result || !result.sql || result.sql.length === 0) { alert('没有生成SQL'); return; }
    const sqlContent = result.sql.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/50 flex items-center justify-center z-50 modal-overlay';
    modal.innerHTML = `
        <div class="modal-content bg-white rounded-2xl shadow-2xl w-full max-w-4xl max-h-[85vh] overflow-hidden mx-4">
            <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200">
                <h3 class="text-lg font-semibold text-slate-900">${result.table}</h3>
                <div class="flex gap-2">
                    <button id="tableDownloadBtn" class="btn btn-primary btn-sm"><i class="fa fa-download"></i>下载 SQL</button>
                    <button id="tableCloseBtn" class="btn btn-ghost btn-sm"><i class="fa fa-times"></i>关闭</button>
                </div>
            </div>
            <div class="p-6 overflow-auto max-h-[60vh] bg-slate-50">
                <pre class="rounded-lg overflow-x-auto text-sm"><code id="tableSqlCode" class="language-sql" style="background: transparent;">${escapeHtml(sqlContent)}</code></pre>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    if (window.hljs) { const el = document.getElementById('tableSqlCode'); el.textContent = sqlContent; window.hljs.highlightElement(el); }
    document.getElementById('tableDownloadBtn').addEventListener('click', () => downloadTableSQL(result.table, result.sql));
    document.getElementById('tableCloseBtn').addEventListener('click', () => modal.remove());
}

function downloadTableSQL(tableName, sqlArray) {
    const blob = new Blob([sqlArray.join('\n\n')], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = `${tableName}_data_compare.sql`;
    document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url);
}

function escapeHtml(text) {
    if (!text) return '';
    const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return text.replace(/[&<>"']/g, m => map[m]);
}

function downloadSQL(targetName, sqlArray) {
    const blob = new Blob([sqlArray.join('\n\n')], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = `${targetName}_data_compare.sql`;
    document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url);
}

init();
