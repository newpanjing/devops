let token = localStorage.getItem('token');
let currentPage = 'dashboard';
let dbConfigs = [];
let sourceConfig = null;
let targetConfigs = [];
let schemaDiff = null;
let dataSyncResults = null;
const DATA_CHANGE_TYPE = {
    INSERT: 'insert',
    UPDATE: 'update',
    DELETE: 'delete'
};

const DATA_CHANGE_TYPE_CONFIG = {
    [DATA_CHANGE_TYPE.INSERT]: {
        label: '新增',
        icon: 'fa-plus-circle',
        colorClass: 'text-green-700',
        backgroundClass: 'bg-green-50',
        borderClass: 'border-green-200'
    },
    [DATA_CHANGE_TYPE.UPDATE]: {
        label: '修改',
        icon: 'fa-pencil-square-o',
        colorClass: 'text-blue-700',
        backgroundClass: 'bg-blue-50',
        borderClass: 'border-blue-200'
    },
    [DATA_CHANGE_TYPE.DELETE]: {
        label: '删除',
        icon: 'fa-trash',
        colorClass: 'text-red-700',
        backgroundClass: 'bg-red-50',
        borderClass: 'border-red-200'
    }
};

async function init() {
    if (token) {
        const valid = await validateToken();
        if (valid) {
            renderApp();
        } else {
            renderLogin();
        }
    } else {
        renderLogin();
    }
}

async function validateToken() {
    try {
        const response = await fetch('/api/auth/validate', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        return result.valid;
    } catch {
        return false;
    }
}

function renderLogin() {
    document.getElementById('app').innerHTML = `
        <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-600 to-indigo-700">
            <div class="bg-white rounded-xl shadow-2xl p-8 w-full max-w-md">
                <div class="text-center mb-6">
                    <div class="flex justify-center mb-4">
                        <div class="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center">
                            <i class="fa fa-database text-blue-600 text-3xl"></i>
                        </div>
                    </div>
                    <h1 class="text-2xl font-bold text-gray-800">数据库同步工具</h1>
                    <p class="text-gray-500 mt-2">请登录以继续</p>
                </div>
                <form id="loginForm" class="space-y-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
                        <input type="text" name="username" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition" placeholder="请输入用户名">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">密码</label>
                        <input type="password" name="password" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition" placeholder="请输入密码">
                    </div>
                    <button type="submit" class="w-full bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center justify-center">
                        <i class="fa fa-sign-in mr-2"></i>登录
                    </button>
                </form>
                <div id="loginError" class="mt-4 text-red-500 text-sm text-center hidden"></div>
                <p class="mt-6 text-center text-gray-500 text-sm">
                    默认账号: admin / admin123
                </p>
            </div>
        </div>
    `;
    document.getElementById('loginForm').addEventListener('submit', handleLogin);
}

async function handleLogin(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        username: form.username.value,
        password: form.password.value
    };
    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.success) {
            token = result.token;
            localStorage.setItem('token', token);
            renderApp();
        } else {
            document.getElementById('loginError').textContent = result.message;
            document.getElementById('loginError').classList.remove('hidden');
        }
    } catch (err) {
        document.getElementById('loginError').textContent = '登录失败，请重试';
        document.getElementById('loginError').classList.remove('hidden');
    }
}

function renderApp() {
    document.getElementById('app').innerHTML = `
        <div class="flex">
            <aside class="sidebar bg-gray-800 text-white w-64 fixed h-full">
                <div class="p-4 border-b border-gray-700">
                    <div class="flex items-center">
                        <i class="fa fa-database text-blue-400 text-xl mr-2"></i>
                        <span class="font-bold text-lg">数据库同步</span>
                    </div>
                </div>
                <nav class="mt-4">
                    <a href="#" onclick="navigate('dashboard')" class="nav-link ${currentPage === 'dashboard' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-home mr-3"></i>仪表盘
                    </a>
                    <a href="#" onclick="navigate('dbconfig')" class="nav-link ${currentPage === 'dbconfig' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-cog mr-3"></i>数据库配置
                    </a>
                    <a href="#" onclick="navigate('schemacompare')" class="nav-link ${currentPage === 'schemacompare' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-table mr-3"></i>表结构比对
                    </a>
                    <a href="#" onclick="navigate('datasync')" class="nav-link ${currentPage === 'datasync' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-refresh mr-3"></i>数据同步
                    </a>
                    <a href="#" onclick="navigate('audit')" class="nav-link ${currentPage === 'audit' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-history mr-3"></i>审计日志
                    </a>
                </nav>
                <div class="absolute bottom-0 w-full p-4 border-t border-gray-700">
                    <button onclick="logout()" class="w-full text-left flex items-center px-4 py-2 hover:bg-gray-700 transition text-gray-400">
                        <i class="fa fa-sign-out mr-3"></i>退出登录
                    </button>
                </div>
            </aside>
            <main class="ml-64 p-6 w-full">
                <div id="mainContent"></div>
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
        case 'dashboard':
            renderDashboard(content);
            break;
        case 'dbconfig':
            await renderDBConfig(content);
            break;
        case 'schemacompare':
            await renderSchemaCompare(content);
            break;
        case 'datasync':
            await renderDataSync(content);
            break;
        case 'audit':
            await renderAudit(content);
            break;
    }
}

function renderDashboard(content) {
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">仪表盘</h1>
            <p class="text-gray-500 mt-1">欢迎使用数据库同步工具</p>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-database text-blue-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">数据库配置</p>
                        <p class="text-2xl font-bold text-gray-800">${dbConfigs.length}</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-table text-green-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">表结构比对</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-yellow-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-refresh text-yellow-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">数据同步</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-history text-purple-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">审计日志</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
        </div>
        <div class="mt-6 bg-white rounded-xl shadow-sm p-6">
            <h2 class="text-lg font-semibold text-gray-800 mb-4">快速开始</h2>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                <button onclick="navigate('dbconfig')" class="p-4 border border-gray-200 rounded-lg hover:border-blue-500 hover:bg-blue-50 transition flex flex-col items-center">
                    <i class="fa fa-cog text-blue-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">添加数据库配置</span>
                    <span class="text-sm text-gray-500">配置源和目标数据库</span>
                </button>
                <button onclick="navigate('schemacompare')" class="p-4 border border-gray-200 rounded-lg hover:border-green-500 hover:bg-green-50 transition flex flex-col items-center">
                    <i class="fa fa-table text-green-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">比对表结构</span>
                    <span class="text-sm text-gray-500">查看源和目标表差异</span>
                </button>
                <button onclick="navigate('datasync')" class="p-4 border border-gray-200 rounded-lg hover:border-yellow-500 hover:bg-yellow-50 transition flex flex-col items-center">
                    <i class="fa fa-refresh text-yellow-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">同步数据</span>
                    <span class="text-sm text-gray-500">将数据同步到目标数据库</span>
                </button>
            </div>
        </div>
    `;
}

async function renderDBConfig(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6 flex justify-between items-center">
            <div>
                <h1 class="text-2xl font-bold text-gray-800">数据库配置</h1>
                <p class="text-gray-500 mt-1">管理数据库连接配置</p>
            </div>
            <button onclick="showAddConfigModal()" class="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                <i class="fa fa-plus mr-2"></i>添加配置
            </button>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6">
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead>
                        <tr class="border-b border-gray-200">
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">名称</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">类型</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">主机</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">数据库</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${dbConfigs.map(config => `
                            <tr class="border-b border-gray-100 hover:bg-gray-50">
                                <td class="py-3 px-4 text-gray-800">${config.name}</td>
                                <td class="py-3 px-4">
                                    <span class="px-2 py-1 text-xs rounded-full ${getDBTypeClass(config.db_type)}">${config.db_type}</span>
                                </td>
                                <td class="py-3 px-4 text-gray-600">${config.host}:${config.port}</td>
                                <td class="py-3 px-4 text-gray-600">${config.database}</td>
                                <td class="py-3 px-4">
                                    <button onclick="testConnection(${config.id})" class="text-blue-600 hover:text-blue-800 mr-3">
                                        <i class="fa fa-check-circle"></i>
                                    </button>
                                    <button onclick="deleteConfig(${config.id})" class="text-red-600 hover:text-red-800">
                                        <i class="fa fa-trash"></i>
                                    </button>
                                </td>
                            </tr>
                        `).join('')}
                        ${dbConfigs.length === 0 ? `
                            <tr>
                                <td colspan="5" class="py-8 text-center text-gray-500">暂无数据库配置</td>
                            </tr>
                        ` : ''}
                    </tbody>
                </table>
            </div>
        </div>
        <div id="configModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center hidden z-50">
            <div class="bg-white rounded-xl p-6 w-full max-w-lg">
                <div class="flex justify-between items-center mb-4">
                    <h2 class="text-xl font-bold text-gray-800">添加数据库配置</h2>
                    <button onclick="hideAddConfigModal()" class="text-gray-400 hover:text-gray-600">
                        <i class="fa fa-times text-xl"></i>
                    </button>
                </div>
                <form id="configForm" class="space-y-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">配置名称</label>
                        <input type="text" name="name" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="例如: 生产数据库">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">数据库类型</label>
                        <select name="db_type" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required>
                            <option value="mysql">MySQL</option>
                            <option value="postgres">PostgreSQL</option>
                            <option value="sqlite">SQLite</option>
                        </select>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">主机</label>
                        <input type="text" name="host" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="localhost">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">端口</label>
                        <input type="number" name="port" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="3306">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">数据库名/路径</label>
                        <input type="text" name="database" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="MySQL/PostgreSQL填数据库名，SQLite填文件路径">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
                        <input type="text" name="username" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="root">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">密码</label>
                        <input type="password" name="password" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="密码">
                    </div>
                    <div class="flex space-x-4">
                        <button type="button" onclick="testNewConnection()" class="flex-1 bg-gray-100 text-gray-700 py-2 rounded-lg font-medium hover:bg-gray-200 transition">
                            测试连接
                        </button>
                        <button type="submit" class="flex-1 bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 transition">
                            保存配置
                        </button>
                    </div>
                </form>
            </div>
        </div>
    `;
    document.getElementById('configForm').addEventListener('submit', handleSaveConfig);
}

function getDBTypeClass(dbType) {
    switch(dbType) {
        case 'mysql': return 'bg-orange-100 text-orange-700';
        case 'postgres': return 'bg-blue-100 text-blue-700';
        case 'sqlite': return 'bg-gray-100 text-gray-700';
        default: return 'bg-gray-100 text-gray-700';
    }
}

function showAddConfigModal() {
    document.getElementById('configModal').classList.remove('hidden');
}

function hideAddConfigModal() {
    document.getElementById('configModal').classList.add('hidden');
    document.getElementById('configForm').reset();
}

async function handleSaveConfig(e) {
    e.preventDefault();
    const form = e.target;
    const formData = new FormData(form);
    const data = {
        name: formData.get('name'),
        db_type: formData.get('db_type'),
        host: formData.get('host'),
        port: parseInt(formData.get('port') || '0'),
        database: formData.get('database'),
        username: formData.get('username'),
        password: formData.get('password')
    };
    try {
        const response = await fetch('/api/dbconfig', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.message) {
            hideAddConfigModal();
            renderDBConfig(document.getElementById('mainContent'));
        }
    } catch (err) {
        alert('保存失败');
    }
}

async function testNewConnection() {
    const form = document.getElementById('configForm');
    const formData = new FormData(form);
    const data = {
        name: formData.get('name'),
        db_type: formData.get('db_type'),
        host: formData.get('host'),
        port: parseInt(formData.get('port') || '0'),
        database: formData.get('database'),
        username: formData.get('username'),
        password: formData.get('password')
    };
    try {
        const response = await fetch('/api/datasync/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.success) {
            alert('连接成功');
        } else {
            alert('连接失败: ' + result.error);
        }
    } catch (err) {
        alert('测试连接失败');
    }
}

async function testConnection(id) {
    const config = dbConfigs.find(c => c.id === id);
    if (!config) return;
    try {
        const response = await fetch('/api/datasync/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        });
        const result = await response.json();
        if (result.success) {
            alert('连接成功');
        } else {
            alert('连接失败: ' + result.error);
        }
    } catch (err) {
        alert('测试连接失败');
    }
}

async function deleteConfig(id) {
    if (!confirm('确定要删除这个配置吗？')) return;
    try {
        const response = await fetch(`/api/dbconfig/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        if (result.message) {
            renderDBConfig(document.getElementById('mainContent'));
        }
    } catch (err) {
        alert('删除失败');
    }
}

async function loadDBConfigs() {
    try {
        const response = await fetch('/api/dbconfig', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        dbConfigs = await response.json();
    } catch {
        dbConfigs = [];
    }
}

async function renderSchemaCompare(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">表结构比对</h1>
            <p class="text-gray-500 mt-1">比对源数据库和目标数据库的表结构差异</p>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">源数据库</label>
                    <select id="sourceConfig" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">目标数据库 (可多选)</label>
                    <select id="targetConfigs" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
            </div>
            <div class="mt-6">
                <button id="schemaCompareBtn" onclick="compareSchemas()" class="bg-blue-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                    <i class="fa fa-search mr-2"></i>开始比对
                </button>
            </div>
        </div>
        <div id="schemaDiffResult"></div>
    `;
}

async function compareSchemas() {
    const sourceID = parseInt(document.getElementById('sourceConfig').value);
    const targetSelect = document.getElementById('targetConfigs');
    const targetIDs = Array.from(targetSelect.selectedOptions).map(o => parseInt(o.value));

    if (!sourceID || targetIDs.length === 0) {
        alert('请选择源数据库和至少一个目标数据库');
        return;
    }

    const resultDiv = document.getElementById('schemaDiffResult');
    const compareButton = document.getElementById('schemaCompareBtn');
    compareButton.disabled = true;
    compareButton.classList.add('opacity-60', 'cursor-not-allowed');
    resultDiv.innerHTML = `
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="flex items-center gap-2 text-blue-600 font-medium">
                <i class="fa fa-spinner fa-spin"></i>正在比对表结构
            </div>
            <div class="mt-4">
                <div class="flex justify-between text-xs text-gray-600 mb-1">
                    <span id="schemaCompareProgressText">进度 0%</span>
                    <span id="schemaCompareTimeText">已用时 00:00 / 剩余时间 --:--</span>
                </div>
                <div class="h-2 bg-gray-200 rounded-full overflow-hidden">
                    <div id="schemaCompareProgressBar" class="h-full bg-blue-600 transition-all" style="width: 0%"></div>
                </div>
            </div>
            <pre id="schemaCompareLogs" class="mt-4 max-h-80 overflow-auto whitespace-pre-wrap bg-gray-900 text-gray-100 p-4 rounded-lg text-xs leading-6"></pre>
        </div>
        <div id="schemaCompareFinalResult"></div>
    `;

    try {
        const response = await fetch('/api/schema/compare/stream', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({ source_id: sourceID, target_ids: targetIDs })
        });
        if (!response.ok) {
            const errorResult = await response.json();
            throw new Error(errorResult.error || '比对失败');
        }

        const results = await readSchemaCompareStream(response);
        renderSchemaDiffResults(results, document.getElementById('schemaCompareFinalResult'));
    } catch (err) {
        const logContainer = document.getElementById('schemaCompareLogs');
        if (logContainer) {
            appendSchemaCompareLog({ level: 'error', message: `比对失败: ${err.message}` });
        } else {
            resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">比对失败: ${err.message}</div>`;
        }
    } finally {
        compareButton.disabled = false;
        compareButton.classList.remove('opacity-60', 'cursor-not-allowed');
    }
}

async function readSchemaCompareStream(response) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let results = null;

    while (true) {
        const { value, done } = await reader.read();
        if (done) {
            break;
        }

        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split('\n\n');
        buffer = events.pop();
        results = processSchemaCompareEvents(events, results);
    }

    if (buffer.trim()) {
        results = processSchemaCompareEvents([buffer], results);
    }
    if (!results) {
        throw new Error('未收到比对结果');
    }
    return results;
}

function processSchemaCompareEvents(events, currentResults) {
    let results = currentResults;
    events.forEach(rawEvent => {
        const parsedEvent = parseSchemaCompareEvent(rawEvent);
        if (!parsedEvent) {
            return;
        }

        if (parsedEvent.name === 'log') {
            appendSchemaCompareLog(parsedEvent.data);
            return;
        }
        if (parsedEvent.name === 'error') {
            throw new Error(parsedEvent.data.message || '比对失败');
        }
        if (parsedEvent.name === 'complete') {
            results = parsedEvent.data.results;
        }
    });
    return results;
}

function parseSchemaCompareEvent(rawEvent) {
    let eventName = 'message';
    const dataLines = [];
    rawEvent.split('\n').forEach(line => {
        if (line.startsWith('event:')) {
            eventName = line.slice('event:'.length).trim();
        }
        if (line.startsWith('data:')) {
            dataLines.push(line.slice('data:'.length).trim());
        }
    });

    if (dataLines.length === 0) {
        return null;
    }

    try {
        return {
            name: eventName,
            data: JSON.parse(dataLines.join('\n'))
        };
    } catch {
        return null;
    }
}

function appendSchemaCompareLog(log) {
    const container = document.getElementById('schemaCompareLogs');
    if (!container) {
        return;
    }

    updateSchemaCompareProgress(log);
    const context = [
        log.target_name ? `目标库: ${log.target_name}` : '',
        log.table_name ? `表: ${log.table_name}` : '',
        log.column_name ? `字段: ${log.column_name}` : ''
    ].filter(Boolean).join(' | ');
    const timestamp = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    container.textContent += `[${timestamp}] ${context ? `${context} | ` : ''}${log.message}\n`;
    container.scrollTop = container.scrollHeight;
}

function updateSchemaCompareProgress(log) {
    if (typeof log.progress !== 'number') {
        return;
    }

    const progress = Math.max(0, Math.min(100, log.progress));
    const progressBar = document.getElementById('schemaCompareProgressBar');
    const progressText = document.getElementById('schemaCompareProgressText');
    const timeText = document.getElementById('schemaCompareTimeText');
    if (progressBar) {
        progressBar.style.width = `${progress.toFixed(1)}%`;
    }
    if (progressText) {
        progressText.textContent = `进度 ${progress.toFixed(1)}%`;
    }
    if (timeText) {
        timeText.textContent = `已用时 ${formatDuration(log.elapsed_seconds || 0)} / 剩余时间 ${formatDuration(log.remaining_seconds || 0)}`;
    }
}

function formatDuration(totalSeconds) {
    if (totalSeconds <= 0) {
        return '00:00';
    }
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

function renderSchemaDiffResults(results, container) {
    if (!results || results.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-gray-500">没有比对结果</div>`;
        return;
    }

    let html = '';
    results.forEach((result, index) => {
        html += `
            <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">目标: ${result.target_name}</h3>
                ${result.error ? `<div class="text-red-500">${result.error}</div>` : ''}
                ${result.sql && result.sql.length > 0 ? `
                    <div class="mt-4">
                        <div class="flex justify-between items-center mb-2">
                            <span class="font-medium text-gray-700">生成的SQL (${result.sql.length}条)</span>
                            <div class="flex gap-2">
                                <button id="schemaExportBtn_${index}" class="bg-purple-600 text-white px-4 py-1.5 rounded-lg text-sm hover:bg-purple-700 transition flex items-center">
                                    <i class="fa fa-file-excel-o mr-1"></i>导出比对结果
                                </button>
                                <button id="schemaDownloadBtn_${index}" class="bg-green-600 text-white px-4 py-1.5 rounded-lg text-sm hover:bg-green-700 transition flex items-center">
                                    <i class="fa fa-download mr-1"></i>下载SQL
                                </button>
                                <button onclick="showSchemaSQLDialog(${index})" class="bg-blue-600 text-white px-4 py-1.5 rounded-lg text-sm hover:bg-blue-700 transition flex items-center">
                                    <i class="fa fa-eye mr-1"></i>查看SQL
                                </button>
                            </div>
                        </div>
                    </div>
                ` : ''}
                ${!result.error ? renderDiff(result.diff) : ''}
            </div>
        `;
    });
    container.innerHTML = html;

    window.currentSchemaResults = results;

    results.forEach((result, index) => {
        const downloadBtn = document.getElementById(`schemaDownloadBtn_${index}`);
        if (downloadBtn) {
            downloadBtn.addEventListener('click', () => {
                downloadSQL(result.target_name, result.sql);
            });
        }
        const exportBtn = document.getElementById(`schemaExportBtn_${index}`);
        if (exportBtn) {
            exportBtn.addEventListener('click', () => {
                exportComparisonResults(result);
            });
        }
    });
}

function renderDiff(diff) {
    if (!diff || !diff.table_diffs || diff.table_diffs.length === 0) {
        return `<div class="text-green-600 text-center py-4"><i class="fa fa-check-circle"></i> 表结构完全一致</div>`;
    }

    let html = '<div class="space-y-4">';
    diff.table_diffs.forEach(tableDiff => {
        let tableHtml = `
            <div class="diff-${tableDiff.type.toLowerCase()} p-4 rounded-lg">
                <div class="flex items-center justify-between mb-2">
                    <span class="font-semibold">${getDiffTypeLabel(tableDiff.type)} 表: ${tableDiff.table_name}</span>
                    <span class="text-xs px-2 py-1 rounded-full ${getDiffTypeBadgeClass(tableDiff.type)}">${tableDiff.type}</span>
                </div>
        `;
        if (tableDiff.column_diffs && tableDiff.column_diffs.length > 0) {
            tableHtml += '<div class="ml-4 space-y-2">';
            tableDiff.column_diffs.forEach(colDiff => {
                tableHtml += `
                    <div class="text-sm">
                        <div class="flex items-center justify-between">
                        <span>${getDiffTypeLabel(colDiff.type)} 列: ${colDiff.column_name}</span>
                        ${colDiff.source_column ? `
                            <span class="text-xs text-gray-500">
                                ${colDiff.source_column.type} ${colDiff.source_column.nullable ? 'NULL' : 'NOT NULL'}
                                ${colDiff.source_column.primary_key ? 'PRIMARY KEY' : ''}
                                ${colDiff.source_column.auto_increment ? 'AUTO_INCREMENT' : ''}
                            </span>
                        ` : ''}
                        </div>
                        ${renderColumnDiffDetail(colDiff)}
                    </div>
                `;
            });
            tableHtml += '</div>';
        }
        if (tableDiff.table_comment_changed) {
            tableHtml += `
                <div class="ml-4 mt-2 text-sm text-gray-600">
                    表备注: ${escapeHtml(tableDiff.target_table ? tableDiff.target_table.comment || '' : '')}
                    <i class="fa fa-arrow-right mx-2"></i>
                    ${escapeHtml(tableDiff.source_table ? tableDiff.source_table.comment || '' : '')}
                </div>
            `;
        }
        tableHtml += '</div>';
        html += tableHtml;
    });
    html += '</div>';

    html += `
        <div class="mt-6 flex justify-end">
            <button onclick="syncSchema()" class="bg-green-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-green-700 transition flex items-center">
                <i class="fa fa-refresh mr-2"></i>同步表结构
            </button>
        </div>
    `;
    schemaDiff = diff;

    return html;
}

function renderColumnDiffDetail(colDiff) {
    if (!colDiff.source_column || !colDiff.target_column) {
        return '';
    }

    const source = colDiff.source_column;
    const target = colDiff.target_column;
    return `
        <div class="mt-1 grid grid-cols-1 md:grid-cols-2 gap-2 text-xs text-gray-500">
            <div>目标: ${escapeHtml(formatColumnSummary(target))}</div>
            <div>源: ${escapeHtml(formatColumnSummary(source))}</div>
        </div>
    `;
}

function formatColumnSummary(column) {
    const parts = [
        column.type,
        column.nullable ? 'NULL' : 'NOT NULL',
        column.default_value ? `DEFAULT ${column.default_value}` : '',
        column.primary_key ? 'PRIMARY KEY' : '',
        column.auto_increment ? 'AUTO_INCREMENT' : '',
        column.comment ? `COMMENT ${column.comment}` : ''
    ];
    return parts.filter(Boolean).join(' ');
}

function getDiffTypeLabel(type) {
    switch(type) {
        case 'CREATE': return '新增';
        case 'ALTER': return '修改';
        case 'DROP': return '删除';
        default: return type;
    }
}

function getDiffTypeBadgeClass(type) {
    switch(type) {
        case 'CREATE': return 'bg-green-100 text-green-700';
        case 'ALTER': return 'bg-yellow-100 text-yellow-700';
        case 'DROP': return 'bg-red-100 text-red-700';
        default: return 'bg-gray-100 text-gray-700';
    }
}

function showSchemaSQLDialog(resultIndex) {
    const result = window.currentSchemaResults[resultIndex];
    if (!result || !result.sql || result.sql.length === 0) {
        alert('没有生成SQL');
        return;
    }

    window.currentSchemaDialogIndex = resultIndex;
    const sqlContent = result.sql.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white rounded-xl shadow-xl w-full max-w-4xl max-h-[85vh] overflow-hidden">
            <div class="flex justify-between items-center p-4 border-b">
                <h3 class="text-lg font-semibold">表结构比对 - ${result.target_name}</h3>
                <button id="schemaCancelBtn" class="bg-gray-200 text-gray-700 px-4 py-1.5 rounded-lg text-sm hover:bg-gray-300 transition">
                    <i class="fa fa-times"></i>取消
                </button>
            </div>
            <div class="p-4 overflow-auto max-h-[65vh]">
                <pre class="bg-gray-100 p-4 rounded-lg overflow-x-auto text-sm text-gray-800">${escapeHtml(sqlContent)}</pre>
            </div>
            <div class="p-4 border-t bg-gray-50 flex justify-end gap-3">
                <button id="schemaDownloadBtn" class="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                    <i class="fa fa-download mr-1"></i>下载SQL
                </button>
                <button id="schemaExecuteBtn" class="bg-green-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-green-700 transition flex items-center">
                    <i class="fa fa-play mr-1"></i>执行SQL
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);

    document.getElementById('schemaCancelBtn').addEventListener('click', () => {
        modal.remove();
    });
    document.getElementById('schemaDownloadBtn').addEventListener('click', () => {
        const idx = window.currentSchemaDialogIndex;
        const res = window.currentSchemaResults[idx];
        downloadSQL(res.target_name, res.sql);
    });
    document.getElementById('schemaExecuteBtn').addEventListener('click', () => {
        executeSchemaSQL(window.currentSchemaDialogIndex);
    });
}

async function executeSchemaSQL(resultIndex) {
    const result = window.currentSchemaResults[resultIndex];
    const sourceID = parseInt(document.getElementById('sourceConfig').value);

    if (!confirm(`确定要执行SQL到目标数据库 ${result.target_name} 吗？此操作不可撤销！`)) {
        return;
    }

    document.querySelector('.fixed').remove();

    try {
        const response = await fetch('/api/schema/execute', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                source_id: sourceID,
                target_id: result.target_id,
                sql: result.sql
            })
        });
        const data = await response.json();

        if (response.ok) {
            alert(`执行完成: ${data.executed}/${data.total} 条成功\n状态: ${data.status}\n${data.errors.length > 0 ? '错误: ' + data.errors.join('\n') : ''}`);
        } else {
            alert('执行失败: ' + (data.error || '未知错误'));
        }
    } catch (err) {
        alert('执行失败: ' + err.message);
    }
}

async function renderDataSync(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">数据同步</h1>
            <p class="text-gray-500 mt-1">选择指定表并比对数据，生成可下载的 SQL 脚本</p>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">源数据库</label>
                    <select id="dataSourceConfig" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" onchange="loadSourceTables()">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">目标数据库 (可多选)</label>
                    <select id="dataTargetConfigs" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
            </div>
            <div class="mt-6">
                <label class="block text-sm font-medium text-gray-700 mb-2">选择需要比对的表</label>
                <select id="dataTables" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                </select>
            </div>
            <div class="mt-6">
                <label class="flex items-center">
                    <input type="checkbox" id="deleteMissing" class="w-4 h-4 text-blue-600 border-gray-300 rounded">
                    <span class="ml-2 text-sm text-gray-700">删除目标数据库中不存在于源数据库的数据</span>
                </label>
            </div>
            <div class="mt-6">
                <button id="dataSyncBtn" onclick="syncData()" class="bg-green-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-green-700 transition flex items-center">
                    <i class="fa fa-search mr-2"></i>比对并生成SQL
                </button>
            </div>
        </div>
        <div id="dataSyncResult"></div>
    `;
}

async function loadSourceTables() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const tableSelect = document.getElementById('dataTables');

    if (!sourceID) {
        tableSelect.innerHTML = '';
        return;
    }

    try {
        const response = await fetch(`/api/schema/tables/${sourceID}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const tables = await response.json();
        tableSelect.innerHTML = tables.map(t => `<option value="${t}">${t}</option>`).join('');
    } catch (err) {
        tableSelect.innerHTML = '<option>加载失败</option>';
    }
}

async function syncData() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const targetSelect = document.getElementById('dataTargetConfigs');
    const targetIDs = Array.from(targetSelect.selectedOptions).map(o => parseInt(o.value));
    const tableSelect = document.getElementById('dataTables');
    const tables = Array.from(tableSelect.selectedOptions).map(o => o.value);
    const deleteMissing = document.getElementById('deleteMissing').checked;

    if (!sourceID || targetIDs.length === 0 || tables.length === 0) {
        alert('请选择源数据库、至少一个目标数据库和需要比对的表');
        return;
    }

    const resultDiv = document.getElementById('dataSyncResult');
    const syncButton = document.getElementById('dataSyncBtn');
    syncButton.disabled = true;
    syncButton.classList.add('opacity-60', 'cursor-not-allowed');
    resultDiv.innerHTML = `
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="flex items-center gap-2 text-blue-600 font-medium">
                <i class="fa fa-spinner fa-spin"></i>正在比对数据并生成SQL
            </div>
            <pre id="dataSyncLogs" class="mt-4 max-h-80 overflow-auto whitespace-pre-wrap bg-gray-900 text-gray-100 p-4 rounded-lg text-xs leading-6"></pre>
        </div>
        <div id="dataSyncFinalResult"></div>
    `;

    try {
        const response = await fetch('/api/datasync/sync/stream', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                source_id: sourceID,
                target_ids: targetIDs,
                tables: tables,
                delete_missing: deleteMissing
            })
        });
        if (!response.ok) {
            const errorResult = await response.json();
            throw new Error(errorResult.error || '比对失败');
        }

        const results = await readDataSyncStream(response);
        renderDataSyncResults(results, document.getElementById('dataSyncFinalResult'));
    } catch (err) {
        const logContainer = document.getElementById('dataSyncLogs');
        if (logContainer) {
            appendDataSyncLog({ level: 'error', message: `生成SQL失败: ${err.message}` });
        } else {
            resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">生成SQL失败: ${err.message}</div>`;
        }
    } finally {
        syncButton.disabled = false;
        syncButton.classList.remove('opacity-60', 'cursor-not-allowed');
    }
}

async function readDataSyncStream(response) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let results = null;

    while (true) {
        const { value, done } = await reader.read();
        if (done) {
            break;
        }

        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split('\n\n');
        buffer = events.pop();
        results = processDataSyncEvents(events, results);
    }

    if (buffer.trim()) {
        results = processDataSyncEvents([buffer], results);
    }
    if (!results) {
        throw new Error('未收到比对结果');
    }
    return results;
}

function processDataSyncEvents(events, currentResults) {
    let results = currentResults;
    events.forEach(rawEvent => {
        const parsedEvent = parseSchemaCompareEvent(rawEvent);
        if (!parsedEvent) {
            return;
        }

        if (parsedEvent.name === 'log') {
            appendDataSyncLog(parsedEvent.data);
            return;
        }
        if (parsedEvent.name === 'error') {
            throw new Error(parsedEvent.data.message || '比对失败');
        }
        if (parsedEvent.name === 'complete') {
            results = parsedEvent.data.results;
        }
    });
    return results;
}

function appendDataSyncLog(log) {
    const container = document.getElementById('dataSyncLogs');
    if (!container) {
        return;
    }

    const context = [
        log.target_name ? `目标库: ${log.target_name}` : '',
        log.table_name ? `表: ${log.table_name}` : ''
    ].filter(Boolean).join(' | ');
    const timestamp = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    container.textContent += `[${timestamp}] ${context ? `${context} | ` : ''}${log.message}\n`;
    container.scrollTop = container.scrollHeight;
}

function renderDataSyncResults(results, container) {
    if (!results || results.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-gray-500">没有同步结果</div>`;
        return;
    }

    let html = '';
    results.forEach((result, index) => {
        const changeSummary = getDataChangeSummary(result.results);
        const allSQL = getAllDataSQL(result.results);

        html += `
            <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">目标: ${result.target_name}</h3>
                ${allSQL.length > 0 ? `
                    <div class="mb-4">
                        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.INSERT, changeSummary[DATA_CHANGE_TYPE.INSERT])}
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.UPDATE, changeSummary[DATA_CHANGE_TYPE.UPDATE])}
                            ${renderDataChangeTypeOption(index, DATA_CHANGE_TYPE.DELETE, changeSummary[DATA_CHANGE_TYPE.DELETE])}
                        </div>
                        <div class="mt-4 flex justify-end gap-2">
                            <button id="dataTypeDownloadBtn_${index}" onclick="downloadSelectedDataSQL(${index})" disabled class="bg-gray-300 text-white px-4 py-2 rounded-lg font-medium cursor-not-allowed transition flex items-center">
                                <i class="fa fa-download mr-1"></i>下载已选SQL
                            </button>
                            <button onclick="showDataSQLDialog(${index})" class="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                                <i class="fa fa-eye mr-1"></i>查看SQL (${allSQL.length}条)
                            </button>
                        </div>
                    </div>
                ` : ''}
                ${result.error ? `<div class="text-red-500">${result.error}</div>` : renderTableResults(result.results, index)}
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
        ? tableChanges.map(change => `<li class="truncate">${escapeHtml(change.table)} (${change.sql.length})</li>`).join('')
        : '<li>无变更</li>';
    const disabled = changeCount === 0 ? 'disabled' : '';

    return `
        <label class="block border ${config.borderClass} ${config.backgroundClass} rounded-lg p-3 ${changeCount === 0 ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'}">
            <div class="flex items-center justify-between gap-3">
                <span class="flex items-center gap-2 font-medium ${config.colorClass}">
                    <input type="checkbox" class="data-change-type-checkbox w-4 h-4" data-target-index="${targetIndex}" data-change-type="${type}" onchange="updateDataSQLDownloadState(${targetIndex})" ${disabled}>
                    <i class="fa ${config.icon}"></i>${config.label}
                </span>
                <span class="text-sm ${config.colorClass}">${changeCount} 条</span>
            </div>
            <ul class="mt-2 text-xs text-gray-600 space-y-1">${tableList}</ul>
        </label>
    `;
}

function getDataChangeSummary(tableResults) {
    const summary = {
        [DATA_CHANGE_TYPE.INSERT]: [],
        [DATA_CHANGE_TYPE.UPDATE]: [],
        [DATA_CHANGE_TYPE.DELETE]: []
    };

    (tableResults || []).forEach(tableResult => {
        const sqlByType = {
            [DATA_CHANGE_TYPE.INSERT]: [],
            [DATA_CHANGE_TYPE.UPDATE]: [],
            [DATA_CHANGE_TYPE.DELETE]: []
        };

        (tableResult.sql || []).forEach(sql => {
            const type = getDataChangeType(sql);
            if (type) {
                sqlByType[type].push(sql);
            }
        });

        Object.keys(sqlByType).forEach(type => {
            if (sqlByType[type].length > 0) {
                summary[type].push({
                    table: tableResult.table,
                    sql: sqlByType[type]
                });
            }
        });
    });

    return summary;
}

function getDataChangeType(sql) {
    const normalizedSQL = sql.trim().toUpperCase();
    if (normalizedSQL.startsWith('INSERT INTO')) {
        return DATA_CHANGE_TYPE.INSERT;
    }
    if (normalizedSQL.startsWith('UPDATE')) {
        return DATA_CHANGE_TYPE.UPDATE;
    }
    if (normalizedSQL.startsWith('DELETE FROM')) {
        return DATA_CHANGE_TYPE.DELETE;
    }
    return null;
}

function getAllDataSQL(tableResults) {
    return (tableResults || []).flatMap(tableResult => tableResult.sql || []);
}

function updateDataSQLDownloadState(targetIndex) {
    const selectedTypes = getSelectedDataChangeTypes(targetIndex);
    const button = document.getElementById(`dataTypeDownloadBtn_${targetIndex}`);
    if (!button) {
        return;
    }

    const enabled = selectedTypes.length > 0;
    button.disabled = !enabled;
    button.classList.toggle('bg-gray-300', !enabled);
    button.classList.toggle('cursor-not-allowed', !enabled);
    button.classList.toggle('bg-green-600', enabled);
    button.classList.toggle('hover:bg-green-700', enabled);
}

function getSelectedDataChangeTypes(targetIndex) {
    const selector = `.data-change-type-checkbox[data-target-index="${targetIndex}"]:checked`;
    return Array.from(document.querySelectorAll(selector)).map(checkbox => checkbox.dataset.changeType);
}

function downloadSelectedDataSQL(targetIndex) {
    const result = window.currentDataResults[targetIndex];
    const selectedTypes = getSelectedDataChangeTypes(targetIndex);
    if (!result || selectedTypes.length === 0) {
        return;
    }

    const selectedTypeSet = new Set(selectedTypes);
    const sql = getAllDataSQL(result.results).filter(statement => selectedTypeSet.has(getDataChangeType(statement)));
    if (sql.length === 0) {
        alert('未找到所选类型的SQL');
        return;
    }

    const typeLabel = selectedTypes.map(type => DATA_CHANGE_TYPE_CONFIG[type].label).join('_');
    downloadSQL(`${result.target_name}_${typeLabel}`, sql);
}

function renderTableResults(tableResults, targetIndex) {
    if (!tableResults || tableResults.length === 0) {
        return `<div class="text-gray-500">没有表数据</div>`;
    }

    let html = '<div class="overflow-x-auto">';
    html += '<table class="w-full">';
    html += `
        <thead>
            <tr class="border-b border-gray-200">
                <th class="text-left py-3 px-4 font-semibold text-gray-600">表名</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">总数</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">新增</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">更新</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">删除</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">状态</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">SQL</th>
            </tr>
        </thead>
        <tbody>
    `;

    tableResults.forEach((result, index) => {
        let statusClass = 'text-green-600';
        let statusIcon = 'fa-check-circle';
        if (result.error) {
            statusClass = 'text-red-600';
            statusIcon = 'fa-times-circle';
        }

        const hasSQL = result.sql && result.sql.length > 0;
        const sqlBtn = hasSQL ? `
            <button onclick="showTableSQL(${targetIndex}, ${index})" class="bg-blue-600 text-white px-3 py-1 rounded-lg text-xs hover:bg-blue-700 transition">
                <i class="fa fa-eye"></i>查看
            </button>
        ` : '-';

        html += `
            <tr class="border-b border-gray-100 hover:bg-gray-50">
                <td class="py-3 px-4 text-gray-800">${result.table}</td>
                <td class="py-3 px-4 text-center text-gray-600">${result.total}</td>
                <td class="py-3 px-4 text-center text-green-600">${result.inserted}</td>
                <td class="py-3 px-4 text-center text-blue-600">${result.updated}</td>
                <td class="py-3 px-4 text-center text-red-600">${result.deleted}</td>
                <td class="py-3 px-4 text-center ${statusClass}">
                    ${result.error ? result.error : `<i class="fa ${statusIcon}"></i>`}
                </td>
                <td class="py-3 px-4 text-center">${sqlBtn}</td>
            </tr>
        `;
    });

    html += '</tbody></table></div>';
    return html;
}

function showDataSQLDialog(resultIndex) {
    const result = window.currentDataResults[resultIndex];
    if (!result || !result.results) {
        alert('没有数据结果');
        return;
    }

    const allSQL = getAllDataSQL(result.results);

    if (allSQL.length === 0) {
        alert('没有生成SQL');
        return;
    }

    window.currentDataDialogIndex = resultIndex;
    const sqlContent = allSQL.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white rounded-xl shadow-xl w-full max-w-4xl max-h-[85vh] overflow-hidden">
            <div class="flex justify-between items-center p-4 border-b">
                <h3 class="text-lg font-semibold">数据同步 - ${result.target_name}</h3>
                <button id="dataCancelBtn" class="bg-gray-200 text-gray-700 px-4 py-1.5 rounded-lg text-sm hover:bg-gray-300 transition">
                    <i class="fa fa-times"></i>取消
                </button>
            </div>
            <div class="p-4 overflow-auto max-h-[65vh]">
                <pre class="bg-gray-100 p-4 rounded-lg overflow-x-auto text-sm text-gray-800">${escapeHtml(sqlContent)}</pre>
            </div>
            <div class="p-4 border-t bg-gray-50 flex justify-end gap-3">
                <button id="dataDownloadBtn" class="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                    <i class="fa fa-download mr-1"></i>下载SQL
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);

    document.getElementById('dataCancelBtn').addEventListener('click', () => {
        modal.remove();
    });
    document.getElementById('dataDownloadBtn').addEventListener('click', () => {
        const idx = window.currentDataDialogIndex;
        const res = window.currentDataResults[idx];
        downloadSQL(res.target_name, getAllDataSQL(res.results));
    });
}

function exportComparisonResults(result) {
    const now = new Date();
    const timestamp = now.toISOString().replace(/[:.]/g, '-');
    const fileName = `schema_compare_${result.target_name}_${timestamp}.json`;

    const exportData = {
        target_name: result.target_name,
        target_id: result.target_id,
        export_time: now.toLocaleString('zh-CN'),
        sql_count: result.sql ? result.sql.length : 0,
        sql: result.sql || [],
        diff: result.diff || {}
    };

    const content = JSON.stringify(exportData, null, 2);
    const blob = new Blob([content], { type: 'application/json' });
    const url = URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

function showTableSQL(targetIndex, tableIndex) {
    const targetResult = window.currentDataResults[targetIndex];
    const result = targetResult && targetResult.results ? targetResult.results[tableIndex] : null;
    if (!result || !result.sql || result.sql.length === 0) {
        alert('没有生成SQL');
        return;
    }

    const sqlContent = result.sql.join('\n\n');
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white rounded-xl shadow-xl w-full max-w-4xl max-h-[80vh] overflow-hidden">
            <div class="flex justify-between items-center p-4 border-b">
                <h3 class="text-lg font-semibold">${result.table} - SQL语句</h3>
                <div class="flex gap-2">
                    <button id="tableDownloadBtn" class="bg-blue-600 text-white px-4 py-1.5 rounded-lg text-sm hover:bg-blue-700 transition">
                        <i class="fa fa-download mr-1"></i>下载SQL
                    </button>
                    <button onclick="this.closest('.fixed').remove()" class="bg-gray-200 text-gray-700 px-4 py-1.5 rounded-lg text-sm hover:bg-gray-300 transition">
                        <i class="fa fa-times"></i>关闭
                    </button>
                </div>
            </div>
            <div class="p-4 overflow-auto max-h-[calc(80vh-60px)]">
                <pre class="bg-gray-100 p-4 rounded-lg overflow-x-auto text-sm text-gray-800">${escapeHtml(sqlContent)}</pre>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('tableDownloadBtn').addEventListener('click', () => {
        downloadTableSQL(result.table, result.sql);
    });
}

function downloadTableSQL(tableName, sqlArray) {
    const sqlContent = sqlArray.join('\n\n');
    const blob = new Blob([sqlContent], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${tableName}_data_sync.sql`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

async function renderAudit(content) {
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">审计日志</h1>
            <p class="text-gray-500 mt-1">查看所有操作记录</p>
        </div>
        <div id="auditLogs"></div>
    `;
    await loadAuditLogs();
}

async function loadAuditLogs() {
    const container = document.getElementById('auditLogs');
    try {
        const response = await fetch('/api/audit/logs?limit=50', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const logs = await response.json();
        renderAuditLogs(logs, container);
    } catch (err) {
        container.innerHTML = `<div class="text-center py-8 text-red-500">加载失败: ${err.message}</div>`;
    }
}

function renderAuditLogs(logs, container) {
    if (!logs || logs.length === 0) {
        container.innerHTML = `<div class="bg-white rounded-xl shadow-sm p-6"><div class="text-center py-8 text-gray-500">暂无审计日志</div></div>`;
        return;
    }

    let html = '';

    logs.forEach(log => {
        let statusClass = 'text-green-600';
        if (log.status === 'FAILED') statusClass = 'text-red-600';
        if (log.status === 'PARTIAL') statusClass = 'text-yellow-600';

        html += `
            <div class="bg-white rounded-xl shadow-sm p-6 mb-4">
                <div class="flex justify-between items-start mb-4">
                    <div>
                        <div class="flex items-center gap-3">
                            <span class="px-2 py-1 text-xs rounded-full bg-blue-100 text-blue-700 font-medium">${log.action}</span>
                            <span class="text-sm text-gray-500">${formatDate(log.created_at)}</span>
                            <span class="text-sm text-gray-500">用户ID: ${log.user_id}</span>
                        </div>
                        <div class="mt-2 text-sm">
                            <span class="text-gray-500">源数据库:</span> <span class="text-gray-800">${log.source_db || '-'}</span>
                            <span class="mx-2">|</span>
                            <span class="text-gray-500">目标数据库:</span> <span class="text-gray-800">${log.target_db || '-'}</span>
                        </div>
                    </div>
                    <div class="text-right">
                        <span class="px-3 py-1 text-xs rounded-full ${log.status === 'SUCCESS' ? 'bg-green-100 text-green-700' : log.status === 'FAILED' ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'} font-medium">${log.status}</span>
                        ${log.applied === 1 ? `<span class="inline-block ml-2 px-2 py-1 text-xs rounded-full bg-purple-100 text-purple-700">已应用</span>` : ''}
                    </div>
                </div>
                <div class="mb-4">
                    <span class="text-sm text-gray-500">详情:</span>
                    <p class="text-gray-800 mt-1">${log.details || '-'}</p>
                </div>
                ${log.sql ? `
                    <div class="border-t border-gray-200 pt-4">
                        <div class="flex justify-between items-center mb-2">
                            <span class="font-medium text-gray-700">SQL语句</span>
                            <button onclick="downloadAuditSQL(${log.id}, '${log.target_db || 'audit'}')" class="bg-blue-600 text-white px-4 py-1.5 rounded-lg text-sm hover:bg-blue-700 transition flex items-center">
                                <i class="fa fa-download mr-1"></i>下载SQL
                            </button>
                        </div>
                        <pre class="bg-gray-100 p-4 rounded-lg overflow-x-auto text-sm text-gray-800 max-h-60 overflow-y-auto">${escapeHtml(log.sql)}</pre>
                        ${log.applied !== 1 ? `
                            <button onclick="applyAuditSQL(${log.id})" class="mt-3 bg-green-600 text-white px-4 py-2 rounded-lg text-sm hover:bg-green-700 transition flex items-center">
                                <i class="fa fa-play mr-1"></i>应用SQL
                            </button>
                        ` : ''}
                    </div>
                ` : ''}
            </div>
        `;
    });

    container.innerHTML = html;
}

function downloadAuditSQL(logID, targetName) {
    fetch(`/api/audit/logs?limit=1&offset=${logID-1}`, {
        headers: { 'Authorization': `Bearer ${token}` }
    }).then(response => response.json()).then(logs => {
        if (logs && logs.length > 0 && logs[0].sql) {
            const blob = new Blob([logs[0].sql], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `audit_${logID}_${targetName}.sql`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        }
    });
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN');
}

function escapeHtml(text) {
    if (!text) return '';
    const map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, function(m) { return map[m]; });
}

function downloadSQL(targetName, sqlArray) {
    const sqlContent = sqlArray.join('\n\n');
    const blob = new Blob([sqlContent], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${targetName}_schema_sync.sql`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

async function applyAuditSQL(logID) {
    if (!confirm('确定要应用这条SQL吗？')) {
        return;
    }

    try {
        const response = await fetch(`/api/audit/apply/${logID}`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });
        const result = await response.json();
        if (response.ok) {
            alert('SQL应用成功');
            loadAuditLogs();
        } else {
            alert('应用失败: ' + result.error);
        }
    } catch (err) {
        alert('应用失败: ' + err.message);
    }
}

init();
