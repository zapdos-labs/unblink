import { type Component, type JSX } from 'solid-js';

interface TabLayoutProps {
  title: string;
  children?: JSX.Element;
  headerAction?: JSX.Element;
}

const TabLayout: Component<TabLayoutProps> = (props) => {
  return (
    <div class="flex flex-col h-screen py-2 overflow-hidden">
      <div class="flex-none h-14 flex items-center px-4 mb-2 bg-neu-900 border-neu-800 border rounded-2xl mr-2">
        <div class="text-lg font-medium">{props.title}</div>
        <div class="flex-1" />
        {props.headerAction}
      </div>
      <div class="flex-1 overflow-hidden">
        <div class="border-neu-800 border rounded-2xl h-full mr-2 bg-neu-900 overflow-hidden max-h-full">
          {props.children}
        </div>
      </div>
    </div>
  );
};

export default TabLayout;
