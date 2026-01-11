import { onMount, onCleanup, createEffect, createSignal, type Component } from 'solid-js';
import { authState, initAuth } from './shared';
import Login from './pages/Login';
import Register from './pages/Register';
import Authorize from './pages/Authorize';
import Authenticated from './components/Authenticated';
import Main from './components/Main';

// Simple router hook
const usePath = () => {
  const [path, setPath] = createSignal(window.location.pathname);

  onMount(() => {
    const handlePopState = () => setPath(window.location.pathname);
    window.addEventListener('popstate', handlePopState);

    onCleanup(() => {
      window.removeEventListener('popstate', handlePopState);
    });
  });

  return path;
};

const App: Component = () => {
  const path = usePath();

  onMount(async () => {
    await initAuth();
    console.log('[App] After initAuth - isLoading:', authState().isLoading, 'isAuthenticated:', authState().isAuthenticated);
  });

  // Redirect to login if not authenticated (reactive)
  createEffect(() => {
    const currentPath = path();
    const isAuthPage = currentPath === '/login' || currentPath === '/login.html' || currentPath === '/register' || currentPath === '/register.html' || currentPath === '/authorize' || currentPath === '/authorize.html';

    if (!isAuthPage && !authState().isLoading && !authState().isAuthenticated) {
      console.log('[App] Redirecting to /login (path:', currentPath, ')');
      window.location.href = '/login';
    }
  });

  // Show auth pages if not authenticated
  const showLoginPage = () => path() === '/login' || path() === '/login.html';
  const showRegisterPage = () => path() === '/register' || path() === '/register.html';
  const showAuthorizePage = () => path() === '/authorize' || path() === '/authorize.html';

  console.log('[App] Rendering - path:', path(), 'isLoading:', authState().isLoading, 'isAuthenticated:', authState().isAuthenticated);

  if (showAuthorizePage()) {
    console.log('[App] Showing authorize page');
    return <Authorize />;
  }

  if (showLoginPage()) {
    console.log('[App] Showing login page');
    return <Login />;
  }

  if (showRegisterPage()) {
    console.log('[App] Showing register page');
    return <Register />;
  }

  console.log('[App] Showing authenticated app');
  return (
    <Authenticated>
      <Main />
    </Authenticated>
  );
};

export default App;
